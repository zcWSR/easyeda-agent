import assert from 'node:assert/strict';
import test from 'node:test';

import * as extensionConfig from '../extension.json';
import { ensureHeaderMenusVisible } from './header-menu';

function stubEda(impl: { insert?: (menus: unknown) => unknown }): { logs: string[] } {
	const logs: string[] = [];
	(globalThis as any).eda = {
		sys_HeaderMenu: {
			insertHeaderMenus: async (menus: unknown) => impl.insert?.(menus),
		},
		sys_Log: { add: (m: string) => logs.push(m) },
	};
	return { logs };
}

// 声明在 extension.json 里还不够:用户安装的扩展,宿主插完菜单从不 flush UI,
// 所以必须在 activate 时用公开接口再插一次(那条路径会 flush)。
test('header menu: the manifest menus are re-inserted through the public API', async () => {
	let received: any;
	stubEda({ insert: (menus) => { received = menus; } });
	try {
		await ensureHeaderMenusVisible(extensionConfig);
		assert.ok(received, 'insertHeaderMenus must be called');
		// 必须原样来自 manifest —— 单一真相来源,运行时不会和 extension.json 走钟。
		assert.deepEqual(received, (extensionConfig as any).headerMenus);
		// 编辑器页面的上下文要在里面,否则打开 PCB/原理图时仍然看不到菜单。
		for (const ctx of ['pcb', 'sch']) {
			assert.ok(Array.isArray(received[ctx]) && received[ctx].length > 0, `${ctx} menus missing`);
		}
		const top = received.pcb[0];
		assert.equal(top.id, 'EDA Agent');
		assert.ok(top.menuItems.some((i: any) => i.registerFn === 'reconnect'),
			'Reconnect is the only daemon-free recovery path; it must be in the menu');
	}
	finally { delete (globalThis as any).eda; }
});

test('header menu: a failing host call never breaks activation', async () => {
	const { logs } = stubEda({ insert: () => { throw new Error('host says no'); } });
	try {
		// 不抛就是通过:activate() 不 await 它,抛出去会变成未处理的 rejection。
		await ensureHeaderMenusVisible(extensionConfig);
		assert.ok(logs.some(l => l.includes('header menu registration failed')),
			'the failure must still be reported to the log panel');
	}
	finally { delete (globalThis as any).eda; }
});

test('header menu: nothing to insert is not an error', async () => {
	let called = false;
	stubEda({ insert: () => { called = true; } });
	try {
		await ensureHeaderMenusVisible({});
		await ensureHeaderMenusVisible(undefined);
		await ensureHeaderMenusVisible(null);
		assert.equal(called, false, 'no headerMenus in the manifest → no host call');
	}
	finally { delete (globalThis as any).eda; }
});
