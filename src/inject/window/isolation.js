// Per-window connection isolation & session state
(function() {
	if (initConn) {
		try {
			sessionStorage.setItem('drizzle_window_conn', JSON.stringify(initConn));
			sessionStorage.removeItem('drizzle_window_empty');
		} catch (e) {}
	} else if (initEmpty) {
		try {
			sessionStorage.setItem('drizzle_window_empty', 'true');
			sessionStorage.removeItem('drizzle_window_conn');
		} catch (e) {}
	} else if (!sessionStorage.getItem('drizzle_window_conn') && !sessionStorage.getItem('drizzle_window_empty')) {
		// Normal session resume: seed this window's session from localStorage
		try {
			const raw = localStorage.getItem('drizzle-gate');
			if (raw) {
				const parsed = JSON.parse(raw);
				if (parsed && parsed.state) {
					if (parsed.state.currentConnection) {
						sessionStorage.setItem('drizzle_window_conn', JSON.stringify(parsed.state.currentConnection));
					} else {
						sessionStorage.setItem('drizzle_window_empty', 'true');
					}
				}
			}
		} catch (e) {}
	}

	// On every page load or F5 reload, restore THIS window's connection into localStorage
	// before React reads it, preventing cross-window state collision.
	try {
		const winConn = sessionStorage.getItem('drizzle_window_conn');
		const winEmpty = sessionStorage.getItem('drizzle_window_empty');

		if (winEmpty === 'true') {
			const raw = localStorage.getItem('drizzle-gate');
			const parsed = raw ? JSON.parse(raw) : { state: {} };
			if (!parsed.state) parsed.state = {};
			parsed.state.currentConnection = null;
			localStorage.setItem('drizzle-gate', JSON.stringify(parsed));
		} else if (winConn) {
			const connObj = JSON.parse(winConn);
			const raw = localStorage.getItem('drizzle-gate');
			const parsed = raw ? JSON.parse(raw) : { state: {} };
			if (!parsed.state) parsed.state = {};
			parsed.state.currentConnection = connObj;
			localStorage.setItem('drizzle-gate', JSON.stringify(parsed));
		}
	} catch (e) {}

	// Continuously keep this window's sessionStorage in sync with any user switching
	try {
		const origSetItem = localStorage.setItem.bind(localStorage);
		localStorage.setItem = function(key, val) {
			if (key === 'drizzle-gate') {
				try {
					const parsed = JSON.parse(val);
					if (parsed && parsed.state) {
						if (parsed.state.currentConnection) {
							sessionStorage.setItem('drizzle_window_conn', JSON.stringify(parsed.state.currentConnection));
							sessionStorage.removeItem('drizzle_window_empty');
						} else {
							sessionStorage.setItem('drizzle_window_empty', 'true');
							sessionStorage.removeItem('drizzle_window_conn');
						}
					}
				} catch (err) {}
			}
			return origSetItem(key, val);
		};
	} catch (e) {}

	// Component synchronization helper
	const targetObj = initConn || (function() {
		try {
			const s = sessionStorage.getItem('drizzle_window_conn');
			return s ? JSON.parse(s) : null;
		} catch (e) { return null; }
	})();

	const isTargetEmpty = initEmpty || (function() {
		try {
			return sessionStorage.getItem('drizzle_window_empty') === 'true';
		} catch (e) { return false; }
	})();

	if (targetObj) {
		let attempts = 0;
		const timer = setInterval(() => {
			attempts++;
			const ds = document.querySelector('drizzle-studio');
			if (ds && typeof ds.setCurrentConnection === 'function') {
				ds.setCurrentConnection(targetObj);
				clearInterval(timer);
				return;
			}
			const all = Array.from(document.querySelectorAll('button, div, span, a'));
			const match = all.find(el => el.textContent && el.textContent.trim() === targetObj.name);
			if (match) {
				match.click();
				clearInterval(timer);
				return;
			}
			if (attempts > 35) clearInterval(timer);
		}, 150);
	} else if (isTargetEmpty) {
		let attempts = 0;
		const timer = setInterval(() => {
			attempts++;
			const ds = document.querySelector('drizzle-studio');
			if (ds && typeof ds.setCurrentConnection === 'function') {
				ds.setCurrentConnection(null);
				clearInterval(timer);
				return;
			}
			const all = Array.from(document.querySelectorAll('button, div, span, a'));
			const backBtn = all.find(el => el.textContent && el.textContent.trim() === 'Back to connections');
			if (backBtn) {
				backBtn.click();
				clearInterval(timer);
				return;
			}
			if (attempts > 40) clearInterval(timer);
		}, 100);
	}
})();
