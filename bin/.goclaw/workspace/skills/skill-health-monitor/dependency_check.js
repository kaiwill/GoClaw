const fs = require('fs');
const path = require('path');

const SKILLS_DIR = path.resolve(__dirname, '..');

// Simple YAML frontmatter parser
function parseFrontMatter(content) {
  const match = content.match(/^---\n([\s\S]*?)\n---/);
  if (!match) return null;
  
  const yamlBlock = match[1];
  const result = {};
  
  const lines = yamlBlock.split('\n');
  let currentKey = null;
  let currentValue = '';

  for (let line of lines) {
    line = line.trim();
    if (!line || line.startsWith('#')) continue;

    // Check for key: value
    const keyMatch = line.match(/^([a-zA-Z0-9_-]+):\s*(.*)$/);
    if (keyMatch) {
      // Save previous
      if (currentKey) {
        try {
          result[currentKey] = JSON.parse(currentValue);
        } catch {
          result[currentKey] = currentValue; // fallback string
        }
      }
      
      currentKey = keyMatch[1];
      currentValue = keyMatch[2];
    } else {
      // Continuation line?
      if (currentKey) {
        currentValue += '\n' + line;
      }
    }
  }
  
  // Save last
  if (currentKey) {
    try {
      result[currentKey] = JSON.parse(currentValue);
    } catch {
      result[currentKey] = currentValue;
    }
  }
  
  return result;
}

function checkDependencies() {
  console.log('🔍 Scanning skill dependencies in:', SKILLS_DIR);
  
  if (!fs.existsSync(SKILLS_DIR)) {
    console.error('❌ Skills directory not found!');
    return;
  }

  const entries = fs.readdirSync(SKILLS_DIR, { withFileTypes: true });
  const skills = entries.filter(ent => ent.isDirectory() && ent.name !== 'node_modules' && !ent.name.startsWith('.'));
  
  const report = {
    total_skills: skills.length,
    missing_env: {},
    missing_bins: {}
  };

  skills.forEach(skill => {
    const skillPath = path.join(SKILLS_DIR, skill.name);
    const skillMdPath = path.join(skillPath, 'SKILL.md');
    
    if (fs.existsSync(skillMdPath)) {
      const content = fs.readFileSync(skillMdPath, 'utf8');
      const meta = parseFrontMatter(content);
      
      // Check for structured metadata (JSON in YAML)
      if (meta && meta.metadata && meta.metadata.openclaw && meta.metadata.openclaw.requires) {
        const requires = meta.metadata.openclaw.requires;
        
        // Check Env
        if (requires.env && Array.isArray(requires.env)) {
          requires.env.forEach(envVar => {
            if (!process.env[envVar]) {
              if (!report.missing_env[skill.name]) report.missing_env[skill.name] = [];
              report.missing_env[skill.name].push(envVar);
            }
          });
        }
      }
    }
  });

  const missingEnvCount = Object.keys(report.missing_env).length;
  
  if (missingEnvCount === 0) {
    console.log('✅ All declared environment variables are present.');
  } else {
    console.log(`\n⚠️  Missing Environment Variables in ${missingEnvCount} skills:`);
    for (const [skill, envs] of Object.entries(report.missing_env)) {
      console.log(` - ${skill}: ${envs.join(', ')}`);
    }
  }

  // Write report
  const reportFile = path.join(__dirname, 'dependency_report.json');
  fs.writeFileSync(reportFile, JSON.stringify(report, null, 2));
  console.log(`💾 Dependency report saved to: ${reportFile}`);
}

if (require.main === module) {
    checkDependencies();
}

module.exports = { checkDependencies };
