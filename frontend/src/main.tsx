import './styles/index.css';

import * as React from 'react';
import ReactDOM from 'react-dom/client';

import App from '@/app';

// Safari keeps a per-origin favicon cache even after the referenced file
// changes. Recreate the icon links with the build commit in the URL so every
// marketplace release has a distinct cache key.
function installBrandFavicon() {
    document.querySelectorAll('link[rel="icon"], link[rel="shortcut icon"]').forEach((link) => link.remove());

    const icon = document.createElement('link');

    icon.rel = 'icon';
    icon.type = 'image/png';
    icon.href = `/favicon/icon-4yi-96.png?v=${GIT_COMMIT_SHA}`;
    document.head.appendChild(icon);
}

installBrandFavicon();

ReactDOM.createRoot(document.querySelector('#root')!).render(
    <React.StrictMode>
        <App />
    </React.StrictMode>,
);
