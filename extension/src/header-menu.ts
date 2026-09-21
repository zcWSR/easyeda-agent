/// <reference types="@jlceda/pro-api-types" />
/**
 * Make the manifest's header menus actually appear.
 *
 * Declaring `headerMenus` in `extension.json` is not enough for a
 * user-installed extension on EasyEDA Pro 3.2.149: the host reads them, but
 * never tells the menu UI about ours. Traced in the desktop app's host bundle
 * (`assets/pro-api/<version>/api.js`), three loaders insert menus:
 *
 * - the per-extension loader inserts and then calls `flushUI()`, which
 *   publishes `/pro-ui/topMenu/advanced/extensionListMenu` — the channel the
 *   Advanced menu actually renders from (`updateExtensionListMenu`);
 * - the builtin bulk loader passes the suppress-flush flag per extension and
 *   then calls `flushUI()` once at the end — also fine;
 * - the **user-installed** bulk loader passes the same suppress-flush flag but
 *   then publishes only `extensionApi.SYS_HeaderMenu.registerHeaderMenus` and
 *   never calls `flushUI()`.
 *
 * So our entries land in the host's registry while the UI is never notified:
 * the `EDA Agent` menu does not appear at all — not even while the connector is
 * connected and serving requests. That costs the only recovery path that does
 * not need the daemon (`Reconnect`), which is exactly what one wants when the
 * connector failed to come up (#221).
 *
 * `eda.sys_HeaderMenu.insertHeaderMenus` goes through the public wrapper, which
 * does call `flushUI()`. Re-inserting the *same* definition is idempotent — the
 * host de-duplicates menu entries by id — so this supplies the missing
 * notification and nothing else.
 *
 * Verified live on 3.2.149 (Windows, international edition): before the call
 * the string `EDA Agent` is absent from the editor DOM; immediately after it,
 * the menu renders under Advanced.
 */

/**
 * Insert the given header menus through the public API so the menu UI is
 * notified. Never throws: a menu is a convenience, and activation must not
 * depend on it.
 *
 * @param config - the parsed `extension.json`; its `headerMenus` is the single
 * source of truth, so the runtime call cannot drift from the manifest.
 */
export async function ensureHeaderMenusVisible(config: unknown): Promise<void> {
	try {
		const menus = (config as { headerMenus?: ISYS_HeaderMenus } | null | undefined)?.headerMenus;
		if (!menus) {
			return;
		}
		await eda.sys_HeaderMenu.insertHeaderMenus(menus);
	}
	catch (err) {
		try {
			eda.sys_Log.add(`[easyeda-agent] header menu registration failed: ${String(err)}`);
		}
		catch { /* log panel unavailable — never let this break activation */ }
	}
}
