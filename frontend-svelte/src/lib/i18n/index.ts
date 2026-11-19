import locales from './locales';

let currentLang =
	(typeof localStorage !== 'undefined' && localStorage.getItem('admin_lang')) || 'cn';

export function t(key: string): string {
	return locales[currentLang]?.[key] || locales['cn']?.[key] || key;
}

export function getLang(): string {
	return currentLang;
}

export function toggleLang(): void {
	currentLang = currentLang === 'cn' ? 'en' : 'cn';
	if (typeof localStorage !== 'undefined') {
		localStorage.setItem('admin_lang', currentLang);
	}
	// Force page reload to update all translated strings
	window.location.reload();
}
