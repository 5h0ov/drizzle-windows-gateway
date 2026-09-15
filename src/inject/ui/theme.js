// Theme initialization
try {
	if (!localStorage.getItem('theme')) {
		localStorage.setItem('theme', themePref);
	}
} catch (e) {}
