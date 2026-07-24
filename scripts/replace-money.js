#!/usr/bin/env node
// Replace all `(value / 100).toFixed(2)` patterns with `fmtMoneyPlain(value)`
// in rockh5 src files. Also adds the `import { fmtMoneyPlain } from '@/lib/money'`.
//
// Run once; do not re-run on already-migrated files.

const fs = require('fs');
const path = require('path');

const files = [
  'src/app/play/baccarat/page.tsx',
  'src/app/play/slot/[id]/page.tsx',
  'src/app/play/poker/[id]/page.tsx',
  'src/app/play/dragon-tiger/page.tsx',
  'src/app/vip/page.tsx',
  'src/app/history/page.tsx',
];

const root = '/home/z/repos/rockh5';

for (const rel of files) {
  const file = path.join(root, rel);
  if (!fs.existsSync(file)) {
    console.log(`SKIP (missing): ${rel}`);
    continue;
  }
  let src = fs.readFileSync(file, 'utf8');
  const orig = src;

  // Match patterns like (X / 100).toFixed(2) where X may have nested parens
  // but is bounded by the outer parentheses. Use a non-greedy capture.
  // Pattern: \( (X) / 100 \)\.toFixed\(2\)
  src = src.replace(/\(\s*([^()]+?)\s*\/\s*100\s*\)\.toFixed\(2\)/g, 'fmtMoneyPlain($1)');

  // history/page.tsx uses `const fmtMoney = (n: number) => (n / 100).toFixed(2);`
  // That's a function definition — delete the local helper since we're importing fmtMoneyPlain.
  src = src.replace(/const fmtMoney = \(n: number\) => \(n \/ 100\)\.toFixed\(2\);\n/, '');

  // Add import if file was modified and doesn't already import fmtMoney
  if (src !== orig && !src.includes('lib/money')) {
    const importMatch = src.match(/^((?:import[^\n]+\n)+)/m);
    if (importMatch) {
      const insertAt = importMatch.index + importMatch[0].length;
      src = src.slice(0, insertAt) + "import { fmtMoney, fmtMoneyPlain } from '@/lib/money';\n" + src.slice(insertAt);
    } else {
      src = "import { fmtMoney, fmtMoneyPlain } from '@/lib/money';\n" + src;
    }
  }

  if (src !== orig) {
    fs.writeFileSync(file, src, 'utf8');
    console.log(`MODIFIED: ${rel}`);
  } else {
    console.log(`NO-CHANGE: ${rel}`);
  }
}

console.log('done');
