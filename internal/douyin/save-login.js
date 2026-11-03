(async () => {
  const visible = el => el.isConnected && el.getClientRects().length > 0 && getComputedStyle(el).visibility !== 'hidden';
  const deadline = Date.now() + 15000;
  let clicked = null;
  let absentSince = null;
  const url = location.href;
  while (Date.now() < deadline) {
    if (location.href !== url) return false;
    const savedSwitches = [...document.querySelectorAll('.trust-login-switch-button')].filter(visible);
    if (savedSwitches.length === 1) return savedSwitches[0].classList.contains('check');
    if (clicked) {
      if (!visible(clicked)) return true;
    } else {
      const prompts = [...document.querySelectorAll('[role="dialog"], dialog, div')].filter(el => {
        const text = (el.innerText || '').trim();
        return visible(el) && text.includes('保存登录信息') && text.length < 500;
      });
      const switches = new Set();
      for (const prompt of prompts) {
        for (const control of prompt.querySelectorAll('[role="switch"], input[type="checkbox"]')) {
          if (visible(control)) switches.add(control);
        }
      }
      if (switches.size === 1) {
        const control = [...switches][0];
        const checked = control.getAttribute('aria-checked');
        if (checked === 'true' || control.checked === true) return true;
        // Leave an off or unreadable switch visible instead of claiming it was saved.
        return false;
      }
      const buttons = new Set();
      for (const prompt of prompts) {
        for (const button of prompt.querySelectorAll('button, [role="button"]')) {
          if (visible(button) && !button.disabled && button.getAttribute('aria-disabled') !== 'true' && /^(保存|保存登录信息|确定)$/.test((button.innerText || '').trim())) buttons.add(button);
        }
      }
      if (prompts.length === 0) {
        absentSince ??= Date.now();
        if (Date.now() - absentSince >= 4000) return true;
      } else {
        absentSince = null;
      }
      if (buttons.size === 1) {
        const button = [...buttons][0];
        clicked = prompts.filter(el => el.contains(button)).sort((a, b) => a.innerText.length - b.innerText.length)[0];
        button.click();
      }
    }
    await new Promise(resolve => setTimeout(resolve, 250));
  }
  return false;
})()
