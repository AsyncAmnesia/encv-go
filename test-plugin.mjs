import { chromium } from 'playwright';
(async () => {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  const consoleMsgs = [];
  page.on('console', m => consoleMsgs.push(`[${m.type()}] ${m.text()}`));
  page.on('pageerror', e => consoleMsgs.push(`[pageerror] ${e.message}`));

  // 1. 打开 plugin-openlist
  await page.goto('http://127.0.0.1:5174/');
  await page.waitForLoadState('networkidle');
  console.log('=== STEP1: /  ===');
  console.log('URL:', page.url());
  console.log('TITLE:', await page.title());

  // 2. 跳转到 #/settings
  await page.goto('http://127.0.0.1:5174/#/settings');
  await page.waitForTimeout(1500);
  console.log('\n=== STEP2: #/settings ===');
  console.log('URL:', page.url());

  // 3. 看 ion-back-button 渲染
  const backBtn = await page.locator('ion-back-button').first();
  const backHtml = await backBtn.evaluate(el => el.outerHTML);
  console.log('\nion-back-button HTML:', backHtml.substring(0, 200));
  const backDefaultHref = await backBtn.getAttribute('default-href');
  console.log('default-href attr:', backDefaultHref);

  // 4. 点击 ion-back-button
  console.log('\n=== STEP3: click ion-back-button ===');
  await backBtn.click();
  await page.waitForTimeout(2000);
  console.log('URL after click:', page.url());

  // 5. 重新到 #/settings 再点 "返回 ENCV 主页面"
  await page.goto('http://127.0.0.1:5174/#/settings');
  await page.waitForTimeout(1500);
  console.log('\n=== STEP4: re-enter /settings, click "返回 ENCV 主页面" ===');
  const backItem = page.locator('ion-item:has-text("返回 ENCV 主页面")').first();
  const exists = await backItem.count();
  console.log('back-to-encv-item exists:', exists);
  if (exists) {
    await backItem.click();
    await page.waitForTimeout(2000);
    console.log('URL after click:', page.url());
    // 查 iframe
    const ifr = page.locator('iframe.encv-iframe');
    const ifrCount = await ifr.count();
    if (ifrCount > 0) {
      const src = await ifr.getAttribute('src');
      console.log('iframe src:', src);
    } else {
      console.log('NO iframe found');
      const bodyHtml = await page.locator('body').innerHTML();
      console.log('body sample (first 500):', bodyHtml.substring(0, 500));
    }
  }

  console.log('\n=== CONSOLE ===');
  for (const m of consoleMsgs) console.log(m);
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
