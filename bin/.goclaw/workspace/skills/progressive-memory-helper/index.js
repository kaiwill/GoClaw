#!/usr/bin/env node
const fs = require('fs');
const path = require('path');
const { program } = require('commander');

// Configuration
const MEMORY_DIR = path.join(process.cwd(), 'memory');
// Ensure memory directory exists
if (!fs.existsSync(MEMORY_DIR)) {
  fs.mkdirSync(MEMORY_DIR, { recursive: true });
}

const TODAY = new Date().toISOString().split('T')[0];
const DAILY_FILE = path.join(MEMORY_DIR, `${TODAY}.md`);

// Token estimator (rough char count / 4)
function estimateTokens(text) {
  return Math.ceil((text || '').length / 4);
}

// Icon map
const TYPE_ICONS = {
  rule: '🚨',
  gotcha: '🔴',
  fix: '🟡',
  how: '🔵',
  change: '🟢',
  discovery: '🟣',
  why: '🟠',
  decision: '🟤',
  tradeoff: '⚖️',
  default: '📝'
};

program
  .version('1.0.0')
  .description('Progressive Memory Helper');

program
  .command('add')
  .description('Add a new memory entry')
  .requiredOption('-t, --type <type>', 'Type (rule, gotcha, fix, change, decision, discovery)')
  .requiredOption('-s, --summary <summary>', 'Short summary for index (max 10 words)')
  .requiredOption('-d, --details <details>', 'Full details/context')
  .option('--content <details>', 'Full details/context (alias for --details)')
  .action((options) => {
    try {
      if (options.content && !options.details) options.details = options.content;
      addEntry(options);
    } catch (error) {
      console.error('Error adding entry:', error.message);
      process.exit(1);
    }
  });

program.parse(process.argv);

function addEntry({ type, summary, details }) {
  let content = '';
  if (fs.existsSync(DAILY_FILE)) {
    content = fs.readFileSync(DAILY_FILE, 'utf8');
  } else {
    // Initialize new file
    content = `# ${TODAY} (Agent Memory)

## 📋 Index (~0 tokens)
| ID | Type | Summary | ~Tok |
|----|------|---------|------|
`;
  }

  const lines = content.split('\n');
  let lastTableLineIndex = -1;
  let nextId = 1;

  // 1. Find the last line of the table and determine next ID
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim();
    if (line.startsWith('|') && line.endsWith('|')) {
      // Table row must be bounded by pipes to be safe
      lastTableLineIndex = i;
      // Parse ID from row: | 1 | ...
      const parts = line.split('|').map(p => p.trim());
      if (parts.length >= 2) {
        const idVal = parseInt(parts[1], 10);
        if (!isNaN(idVal) && idVal >= nextId) {
          nextId = idVal + 1;
        }
      }
    } else if (lastTableLineIndex !== -1 && line === '') {
      // Table ended by empty line - stop scanning to avoid picking up detail blocks
      break; 
    }
  }

  // If no table found, we must be in a fresh file or broken state
  // If fresh file (created above), lastTableLineIndex should point to `|----|...` (line 4 or 5)
  if (lastTableLineIndex === -1) {
      // If we just created the file, lines are few.
      // Line 0: # Date...
      // Line 1: empty
      // Line 2: ## Index...
      // Line 3: | ID | ...
      // Line 4: |----| ...
      // So lastTableLineIndex should be ~4.
      // Let's re-scan specifically for the header separator if we missed it.
      for (let i = 0; i < lines.length; i++) {
          if (lines[i].includes('|----|')) {
              lastTableLineIndex = i;
              break;
          }
      }
  }

  const icon = TYPE_ICONS[type] || TYPE_ICONS.default; // Fallback to default icon
  const detailTokens = estimateTokens(details);
  
  // Construct new row
  // | 1 | 🔴 | Summary | 80 |
  const newRow = `| ${nextId} | ${icon} | ${summary} | ${detailTokens} |`;

  // Construct detail block
  const detailBlock = `\n### #${nextId} | ${icon} ${summary} | ~${detailTokens} tokens\n${details}\n`;

  // Insert row into table
  if (lastTableLineIndex !== -1 && lastTableLineIndex < lines.length) {
    lines.splice(lastTableLineIndex + 1, 0, newRow);
  } else {
    // Fallback: append to end if structure is totally lost
    lines.push(newRow);
  }

  // Join lines back
  let newContent = lines.join('\n');

  // Append detail block at the very end of the file
  newContent += detailBlock;

  fs.writeFileSync(DAILY_FILE, newContent, 'utf8');
  console.log(`[Progressive Memory] Added entry #${nextId} to ${DAILY_FILE}`);
}
