// Full-Row & Multi-Row Copy Patch (Complete Schema & React Fiber)
(function() {
	let lastRightClickedRow = null;

	function getDrizzleRoot() {
		const ds = document.querySelector('drizzle-studio');
		if (ds && ds.shadowRoot) {
			return ds.shadowRoot;
		}
		return document;
	}

	function findRowFromElement(el) {
		if (!el) return null;
		const direct = el.closest ? el.closest('[role="row"]') : null;
		if (direct) return direct;

		try {
			const rect = el.getBoundingClientRect ? el.getBoundingClientRect() : null;
			if (rect && rect.width > 0) {
				const root = getDrizzleRoot();
				const probeX = rect.right + 30;
				const probeY = rect.top + (rect.height / 2);
				const probeEl = (root.elementFromPoint ? root.elementFromPoint(probeX, probeY) : null) || document.elementFromPoint(probeX, probeY);
				if (probeEl) {
					const found = probeEl.closest ? probeEl.closest('[role="row"]') : null;
					if (found) return found;
				}
			}
		} catch (e) {}
		return null;
	}

	function getDataGridAllRows(root) {
		try {
			const grid = root.querySelector('[role="grid"]');
			if (!grid) return null;
			const fiberKey = Object.keys(grid).find(function(k) { return k.startsWith('__reactFiber$'); });
			if (!fiberKey || !grid[fiberKey]) return null;
			let fiber = grid[fiberKey];
			while (fiber) {
				const props = fiber.memoizedProps || fiber.pendingProps;
				if (props && Array.isArray(props.rows) && props.rows.length > 0) {
					return props.rows;
				}
				fiber = fiber.return;
			}
		} catch (e) {}
		return null;
	}

	function getFullRecordFromRow(rowEl, root, allGridRows) {
		if (!rowEl) return null;

		// Strategy 1: DataGrid rows array lookup via aria-rowindex
		try {
			const rowIdxAttr = rowEl.getAttribute('aria-rowindex');
			if (rowIdxAttr && allGridRows && allGridRows.length > 0) {
				// In react-data-grid, header row is index 1, so row 0 is index 2
				const rowIdx = parseInt(rowIdxAttr, 10) - 2;
				if (rowIdx >= 0 && rowIdx < allGridRows.length) {
					const item = allGridRows[rowIdx];
					if (item) {
						if (item.values && typeof item.values === 'object') {
							return item.values;
						}
						if (typeof item === 'object') {
							return item;
						}
					}
				}
			}
		} catch (e) {}

		// Strategy 2: React Fiber traversal on row element & cells
		const candidates = [rowEl, rowEl.firstElementChild, rowEl.querySelector('[role="gridcell"]')];
		for (let i = 0; i < candidates.length; i++) {
			const el = candidates[i];
			if (!el) continue;
			const fiberKey = Object.keys(el).find(function(k) { return k.startsWith('__reactFiber$'); });
			if (fiberKey && el[fiberKey]) {
				let fiber = el[fiberKey];
				while (fiber) {
					const props = fiber.memoizedProps || fiber.pendingProps;
					if (props) {
						if (props.row) {
							if (props.row.values && typeof props.row.values === 'object') {
								return props.row.values;
							}
							if (typeof props.row === 'object' && !props.row.$$typeof) {
								if (props.row.id !== undefined || Object.keys(props.row).length > 2) {
									return props.row;
								}
							}
						}
						if (props.record && typeof props.record === 'object') {
							return props.record.values || props.record;
						}
					}
					fiber = fiber.return;
				}
			}
		}
		return null;
	}

	window.addEventListener('contextmenu', function(e) {
		const path = e.composedPath ? e.composedPath() : [e.target];
		lastRightClickedRow = null;
		for (let i = 0; i < path.length; i++) {
			const found = findRowFromElement(path[i]);
			if (found) {
				lastRightClickedRow = found;
				break;
			}
		}
	}, true);

	window.addEventListener('mousedown', function(e) {
		if (e.button === 2) {
			const path = e.composedPath ? e.composedPath() : [e.target];
			for (let i = 0; i < path.length; i++) {
				const found = findRowFromElement(path[i]);
				if (found) {
					lastRightClickedRow = found;
					break;
				}
			}
		}
	}, true);

	function copyTextToClipboard(str) {
		if (!str) return;
		if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
			navigator.clipboard.writeText(str).catch(function() {
				fallbackCopyText(str);
			});
		} else {
			fallbackCopyText(str);
		}
	}

	function fallbackCopyText(str) {
		try {
			const ta = document.createElement('textarea');
			ta.value = str;
			ta.style.position = 'fixed';
			ta.style.left = '-9999px';
			ta.style.top = '-9999px';
			document.body.appendChild(ta);
			ta.focus();
			ta.select();
			document.execCommand('copy');
			document.body.removeChild(ta);
		} catch (e) {}
	}

	document.addEventListener('click', function(e) {
		const path = e.composedPath ? e.composedPath() : [e.target];
		let actionType = null;
		for (let i = 0; i < path.length; i++) {
			const el = path[i];
			if (!el || !el.textContent) continue;
			const txt = el.textContent.trim();
			if (txt.startsWith('Copy') && txt.includes('Ctrl+C')) {
				actionType = 'copy_all';
				break;
			} else if (txt.includes('Copy as JSON')) {
				actionType = 'copy_json';
				break;
			} else if (txt.includes('Copy as CSV')) {
				actionType = 'copy_csv';
				break;
			} else if (txt.includes('Copy as SQL')) {
				actionType = 'copy_sql';
				break;
			}
		}
		if (!actionType) return;

		const root = getDrizzleRoot();
		const allGridRows = getDataGridAllRows(root);

		// Determine target rows: checked rows if multiple, or right-clicked row
		const checkedRows = Array.from(root.querySelectorAll('[role="row"][aria-selected="true"]'));
		let rowsToProcess = [];
		if (checkedRows.length > 0 && lastRightClickedRow && checkedRows.includes(lastRightClickedRow)) {
			rowsToProcess = checkedRows;
		} else if (lastRightClickedRow) {
			rowsToProcess = [lastRightClickedRow];
		} else if (checkedRows.length > 0) {
			rowsToProcess = checkedRows;
		}

		if (rowsToProcess.length === 0) return;

		// Fallback column header names from DOM
		const headerRow = root.querySelector('[role="row"][aria-rowindex="1"]') || root.querySelector('.rdg-header-row');
		const headerCells = headerRow ? Array.from(headerRow.querySelectorAll('[role="columnheader"]')) : [];
		const domColNames = [];
		headerCells.forEach(function(hc) {
			const nameSpan = hc.querySelector('.text-muted-foreground');
			const name = (nameSpan ? nameSpan.textContent : hc.textContent || '').trim();
			if (name) domColNames.push(name);
		});

		// Process rows using React Fiber (for 100% complete schema columns), fallback to DOM cells
		const parsedRecords = [];
		rowsToProcess.forEach(function(rowEl) {
			const fiberValues = getFullRecordFromRow(rowEl, root, allGridRows);
			if (fiberValues && typeof fiberValues === 'object') {
				parsedRecords.push(fiberValues);
			} else {
				// DOM fallback for visible cells
				const gridCells = Array.from(rowEl.querySelectorAll('[role="gridcell"]'));
				let dataCells = gridCells;
				if (gridCells.length > domColNames.length) {
					dataCells = gridCells.slice(gridCells.length - domColNames.length);
				}
				const obj = {};
				domColNames.forEach(function(colName, idx) {
					const cell = dataCells[idx];
					const val = cell ? cell.innerText.trim() : '';
					obj[colName] = val === 'NULL' ? null : val;
				});
				parsedRecords.push(obj);
			}
		});

		if (parsedRecords.length === 0) return;

		const sampleCols = Object.keys(parsedRecords[0]);
		let textToCopy = '';

		if (actionType === 'copy_all') {
			// Tab-separated full rows
			textToCopy = parsedRecords.map(function(r) {
				return sampleCols.map(function(k) {
					const v = r[k];
					return v === null ? 'NULL' : (typeof v === 'object' ? JSON.stringify(v) : String(v));
				}).join('\t');
			}).join('\n');
		} else if (actionType === 'copy_json') {
			if (parsedRecords.length === 1) {
				textToCopy = JSON.stringify(parsedRecords[0], null, 2);
			} else {
				textToCopy = JSON.stringify(parsedRecords, null, 2);
			}
		} else if (actionType === 'copy_csv') {
			const csvLines = parsedRecords.map(function(r) {
				return sampleCols.map(function(k) {
					const v = r[k];
					const strVal = v == null ? '' : (typeof v === 'object' ? JSON.stringify(v) : String(v));
					return '"' + strVal.replace(/"/g, '""') + '"';
				}).join(',');
			});
			textToCopy = [sampleCols.join(','), ...csvLines].join('\n');
		} else if (actionType === 'copy_sql') {
			const tableHeader = root.querySelector('h1, [data-testid="table-name"], .truncate.font-medium');
			const tableName = tableHeader ? tableHeader.textContent.trim() : 'table_name';
			textToCopy = parsedRecords.map(function(r) {
				const cols = sampleCols.join(', ');
				const vals = sampleCols.map(function(k) {
					const v = r[k];
					if (v === null || v === undefined) return 'NULL';
					const strVal = typeof v === 'object' ? JSON.stringify(v) : String(v);
					return "'" + strVal.replace(/'/g, "''") + "'";
				}).join(', ');
				return 'INSERT INTO ' + tableName + ' (' + cols + ') VALUES (' + vals + ');';
			}).join('\n');
		}

		if (textToCopy) {
			copyTextToClipboard(textToCopy);
			showCopyToast(parsedRecords.length === 1 ? 'Copied 1 row' : 'Copied ' + parsedRecords.length + ' rows');
		}
	}, true);
})();
