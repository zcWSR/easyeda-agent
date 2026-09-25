/// <reference types="@jlceda/pro-api-types" />

import assert from 'node:assert/strict';
import { test } from 'node:test';
import JSZip from 'jszip';

import { resolveNativeLocalDevice } from './actions';
import { readProjectNativeAssetSourceArchive } from './native-footprint-source';
import { projectNativeAssetSourceInventory, readNativeProjectAssetSource } from './util';

const pageUuid = '1234567890abcdef';
const libraryUuid = 'sample-local-library';
const instance = { device: '1111111111111111', symbol: '2222222222222222', footprint: '3333333333333333' };
const asset = { device: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', symbol: 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', footprint: 'cccccccccccccccccccccccccccccccc' };
const row = (type: string, payload: Record<string, unknown>) => `${JSON.stringify({ type })}||${JSON.stringify(payload)}|\n`;
const source = (type: 'DEVICE' | 'SYMBOL' | 'FOOTPRINT', id: string, uuid: string, library = libraryUuid) =>
	row('DOCHEAD', { docType: type, uuid: id }) + row('META', { source: `${uuid}|${library}`, title: 'EA_AGENT__COPPER_2P' });
const project = source('DEVICE', instance.device, asset.device)
	+ source('SYMBOL', instance.symbol, asset.symbol)
	+ source('FOOTPRINT', instance.footprint, asset.footprint)
	+ row('DOCHEAD', { docType: 'SCH_PAGE', uuid: pageUuid });
const snapshot: Record<string, unknown> = {
	addIntoBom: false, addIntoPcb: true, name: 'EA_AGENT__COPPER_2P', supplierId: '', manufacturerId: '',
	device: { uuid: instance.device, libraryUuid, name: 'EA_AGENT__COPPER_2P' },
	symbol: { uuid: instance.symbol, libraryUuid },
	footprint: { uuid: instance.footprint, libraryUuid, name: 'COPPER_2P' },
};
const detail: Record<string, unknown> = {
	uuid: asset.device, name: 'EA_AGENT__COPPER_2P', property: { name: 'EA_AGENT__COPPER_2P', addIntoBom: false, addIntoPcb: true },
	association: { symbol: { uuid: asset.symbol, libraryUuid: null }, footprint: { uuid: asset.footprint, libraryUuid: null } },
};

test('native local copper resolver proves exact DEVICE/SYMBOL/FOOTPRINT sources and official association', async () => {
	const inventory = projectNativeAssetSourceInventory(project, pageUuid);
	assert.equal(inventory.length, 3);
	assert.deepEqual(readNativeProjectAssetSource(inventory, 'DEVICE', instance.device).source, {
		instanceUuid: instance.device, uuid: asset.device, libraryUuid, sourceKind: 'project-epro2',
	});
	let queried = '';
	const result = await resolveNativeLocalDevice(snapshot,
		async () => ({ entries: inventory }),
		async (uuid, library) => { queried = `${uuid}@${library}`; return detail; });
	assert.equal(queried, `${asset.device}@${libraryUuid}`);
	assert.deepEqual(result.device, { uuid: asset.device, libraryUuid, via: 'native-local-copper-source' });
	assert.equal(result.footprintSource?.uuid, asset.footprint);
});

test('native local copper archive parser binds the current page and rejects stale page', async () => {
	const zip = new JSZip(); zip.file('one.epru', project); zip.file('project2.json', '{}');
	const archive = new Blob([await zip.generateAsync({ type: 'arraybuffer', compression: 'DEFLATE' })]);
	assert.equal((await readProjectNativeAssetSourceArchive(archive, pageUuid)).length, 3);
	await assert.rejects(readProjectNativeAssetSourceArchive(archive, '0000000000000000'), /matching SCH_PAGE/);
});

test('native local copper resolver refuses ambiguous or conflicting evidence', async (t) => {
	const inventory = projectNativeAssetSourceInventory(project, pageUuid);
	const cases: Array<[string, Record<string, unknown>, typeof inventory, Record<string, unknown>]> = [
		['non-BOM flag changed', { ...snapshot, addIntoBom: true }, inventory, detail],
		['supplier id appeared', { ...snapshot, supplierId: 'C123' }, inventory, detail],
		['instance footprint changed', { ...snapshot, footprint: { uuid: '4444444444444444', libraryUuid } }, inventory, detail],
		['duplicate device source', snapshot, [...inventory, inventory[0]], detail],
		['device name changed', snapshot, inventory, { ...detail, name: 'OTHER' }],
		['official footprint association differs', snapshot, inventory, { ...detail, association: { ...detail.association as object, footprint: { uuid: 'dddddddddddddddddddddddddddddddd' } } }],
		['official symbol association differs', snapshot, inventory, { ...detail, association: { ...detail.association as object, symbol: { uuid: 'dddddddddddddddddddddddddddddddd' } } }],
		['official BOM flag differs', snapshot, inventory, { ...detail, property: { ...detail.property as object, addIntoBom: true } }],
	];
	for (const [name, placed, sources, official] of cases) {
		await t.test(name, async () => {
			const result = await resolveNativeLocalDevice(placed, async () => ({ entries: sources }), async () => official);
			assert.equal(result.device, undefined);
			assert.match(result.reason ?? '', /identity unresolved/);
		});
	}
	await t.test('wrong native source library', async () => {
		const wrong = projectNativeAssetSourceInventory(project.replace(`${asset.footprint}|${libraryUuid}`, `${asset.footprint}|other-library`), pageUuid);
		const result = await resolveNativeLocalDevice(snapshot, async () => ({ entries: wrong }), async () => detail);
		assert.equal(result.device, undefined);
	});
});

const bomSnapshot: Record<string, unknown> = {
	...snapshot, addIntoBom: true, name: '={Value}',
	component: { libraryUuid, uuid: instance.device, name: 'EA_AGENT__COPPER_2P' },
	manufacturer: 'TECH PUBLIC', manufacturerId: 'B140WS-TP', supplier: 'LCSC', supplierId: 'C55113741',
};
const bomProperty = {
	name: 'EA_AGENT__COPPER_2P', addIntoBom: true, addIntoPcb: true,
	manufacturer: 'TECH PUBLIC', manufacturerId: 'B140WS-TP', supplier: 'LCSC', supplierId: 'C55113741',
	otherProperty: { Manufacturer: 'TECH PUBLIC', 'Manufacturer Part': 'B140WS-TP', Supplier: 'LCSC', 'Supplier Part': 'C55113741' },
};

test('native local BOM resolver proves exact procurement fields without online LCSC search', async () => {
	const inventory = projectNativeAssetSourceInventory(project, pageUuid);
	const result = await resolveNativeLocalDevice(bomSnapshot, async () => ({ entries: inventory }), async () => ({ ...detail, property: bomProperty }));
	assert.deepEqual(result.device, { uuid: asset.device, libraryUuid, via: 'native-local-bom-source' });
	assert.equal(result.lcsc, 'C55113741');
});

test('native local DNP resolver keeps non-BOM assembly policy and proves procurement identity', async () => {
	const inventory = projectNativeAssetSourceInventory(project, pageUuid);
	const placed = { ...bomSnapshot, addIntoBom: false };
	const official = { ...bomProperty, addIntoBom: false };
	const result = await resolveNativeLocalDevice(placed, async () => ({ entries: inventory }), async () => ({ ...detail, property: official }));
	assert.deepEqual(result.device, { uuid: asset.device, libraryUuid, via: 'native-local-dnp-source' });
	assert.equal(result.lcsc, 'C55113741');
});

test('native local BOM resolver refuses missing or conflicting procurement and binding evidence', async (t) => {
	const inventory = projectNativeAssetSourceInventory(project, pageUuid);
	const cases: Array<[string, Record<string, unknown>, typeof inventory, Record<string, unknown>]> = [
		['missing native device source', bomSnapshot, inventory.filter(item => item.docType !== 'DEVICE'), { ...detail, property: bomProperty }],
		['placed MPN differs', { ...bomSnapshot, manufacturerId: 'OTHER' }, inventory, { ...detail, property: bomProperty }],
		['placed C-number differs', { ...bomSnapshot, supplierId: 'C999' }, inventory, { ...detail, property: bomProperty }],
		['placed manufacturer differs', { ...bomSnapshot, manufacturer: 'OTHER' }, inventory, { ...detail, property: bomProperty }],
		['placed supplier differs', { ...bomSnapshot, supplier: 'OTHER' }, inventory, { ...detail, property: bomProperty }],
		['placed BOM flag differs', { ...bomSnapshot, addIntoBom: false }, inventory, { ...detail, property: bomProperty }],
		['official BOM flag differs', bomSnapshot, inventory, { ...detail, property: { ...bomProperty, addIntoBom: false } }],
		['official MPN differs', bomSnapshot, inventory, { ...detail, property: { ...bomProperty, manufacturerId: 'OTHER' } }],
		['official C-number differs', bomSnapshot, inventory, { ...detail, property: { ...bomProperty, supplierId: 'C999' } }],
		['otherProperty C-number differs', bomSnapshot, inventory, { ...detail, property: { ...bomProperty, otherProperty: { ...bomProperty.otherProperty, 'Supplier Part': 'C999' } } }],
		['symbol binding differs', bomSnapshot, inventory, { ...detail, property: bomProperty, association: { ...detail.association as object, symbol: { uuid: 'dddddddddddddddddddddddddddddddd' } } }],
		['footprint binding differs', bomSnapshot, inventory, { ...detail, property: bomProperty, association: { ...detail.association as object, footprint: { uuid: 'dddddddddddddddddddddddddddddddd' } } }],
	];
	for (const [name, placed, sources, official] of cases) {
		await t.test(name, async () => {
			const result = await resolveNativeLocalDevice(placed, async () => ({ entries: sources }), async () => official);
			assert.equal(result.device, undefined);
			assert.match(result.reason ?? '', /identity unresolved/);
		});
	}
});
