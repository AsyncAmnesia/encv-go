import { root } from '@lynx-js/react';
import { AppComponent } from './components/AppComponent';

root.render(<AppComponent />);

if (import.meta.webpackHot) {
  import.meta.webpackHot.accept();
}
