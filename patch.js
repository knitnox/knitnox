const fs = require('fs');
let content = fs.readFileSync('src/lib/components/SettingsModal.svelte', 'utf8');

content = content.replace(
    '<div class="absolute -top-3 left-6 z-10">',
    '<div class="absolute -top-3 left-1/2 -translate-x-1/2 z-10 whitespace-nowrap">'
);

content = content.replace(
    /<\/button>\s*<\/div>\s*<\/section>\s*<\/div>\s*<\/div>\s*<!-- Visuals Section -->/,
    '</button>\n\t\t\t\t\t\t\t</div>\n\t\t\t\t\t\t</section>\n\n\t\t\t\t\t\t<!-- Visuals Section -->'
);

content = content.replace(
    /<input type="file" accept="image\/\*" class="hidden" bind:this=\{qrImportInput\} onchange=\{handleQRImport\} \/>\s*<\/div>\s*<\/section>\s*\{\/if\}/,
    '<input type="file" accept="image/*" class="hidden" bind:this={qrImportInput} onchange={handleQRImport} />\n\t\t\t\t\t\t</div>\n\t\t\t\t\t</section>\n\t\t\t\t\t</div>\n\t\t\t\t\t</div>\n\t\t\t\t{/if}'
);

fs.writeFileSync('src/lib/components/SettingsModal.svelte', content);
console.log('Patch complete.');
