// Drizzle-styled dark toast notification
function showCopyToast(msg) {
	try {
		let tip = document.getElementById('drizzle-gw-copy-toast');
		if (!tip) {
			tip = document.createElement('div');
			tip.id = 'drizzle-gw-copy-toast';
			tip.style.position = 'fixed';
			tip.style.bottom = '24px';
			tip.style.left = '50%';
			tip.style.transform = 'translateX(-50%) translateY(0)';
			tip.style.backgroundColor = '#18181b'; // zinc-900
			tip.style.color = '#f4f4f5'; // zinc-100
			tip.style.border = '1px solid #27272a'; // zinc-800
			tip.style.padding = '8px 16px';
			tip.style.borderRadius = '8px';
			tip.style.boxShadow = '0 10px 25px -5px rgba(0, 0, 0, 0.5), 0 8px 10px -6px rgba(0, 0, 0, 0.5)';
			tip.style.fontSize = '13px';
			tip.style.fontWeight = '500';
			tip.style.fontFamily = 'ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif';
			tip.style.zIndex = '99999999';
			tip.style.display = 'flex';
			tip.style.alignItems = 'center';
			tip.style.gap = '8px';
			tip.style.transition = 'opacity 0.18s cubic-bezier(0.16, 1, 0.3, 1), transform 0.18s cubic-bezier(0.16, 1, 0.3, 1)';
			tip.style.pointerEvents = 'none';

			// Icon
			const icon = document.createElement('span');
			icon.innerHTML = '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#34d399" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>';
			icon.style.display = 'inline-flex';
			icon.style.alignItems = 'center';

			const text = document.createElement('span');
			text.id = 'drizzle-gw-toast-text';

			tip.appendChild(icon);
			tip.appendChild(text);
			document.body.appendChild(tip);
		}

		const textSpan = tip.querySelector('#drizzle-gw-toast-text');
		if (textSpan) textSpan.textContent = msg;

		tip.style.opacity = '1';
		tip.style.transform = 'translateX(-50%) translateY(0)';
		clearTimeout(tip._timer);
		tip._timer = setTimeout(function() {
			tip.style.opacity = '0';
			tip.style.transform = 'translateX(-50%) translateY(8px)';
		}, 2000);
	} catch (e) {}
}
