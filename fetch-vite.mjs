// 直接拉 vite 编译后的 entry + 看 ion-back-button 处理逻辑
import { JSDOM } from 'jsdom';
import http from 'http';

function fetchUrl(url) {
  return new Promise((resolve, reject) => {
    http.get(url, res => {
      let data = '';
      res.on('data', d => data += d);
      res.on('end', () => resolve({ status: res.statusCode, body: data, headers: res.headers }));
    }).on('error', reject);
  });
}

(async () => {
  // 1. 看 index.html
  const html = await fetchUrl('http://127.0.0.1:5174/');
  console.log('=== index.html ===');
  console.log(html.body);
  console.log('\n=== base tag ===');
  const baseMatch = html.body.match(/<base[^>]+>/);
  console.log(baseMatch ? baseMatch[0] : 'NO base tag');
})();
