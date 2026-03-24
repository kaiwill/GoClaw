const fs = require('fs');
const path = require('path');
const dependencyCheck = require('./dependency_check.js');

const SKILLS_DIR = path.resolve(__dirname, '..');

function checkSkills() {
  console.log('🔍 Scanning skills directory:', SKILLS_DIR);
  
  if (!fs.existsSync(SKILLS_DIR)) {
    console.error('❌ Skills directory not found!');
    return;
  }

  const entries = fs.readdirSync(SKILLS_DIR, { withFileTypes: true });
  const skills = entries.filter(ent => ent.isDirectory() && ent.name !== 'node_modules' && !ent.name.startsWith('.'));
  
  const report = {
    total: skills.length,
    healthy: 0,
    broken: [],
    missing_docs: [],
    missing_package: []
  };

  skills.forEach(skill => {
    const skillPath = path.join(SKILLS_DIR, skill.name);
    const hasSkillMd = fs.existsSync(path.join(skillPath, 'SKILL.md'));
    const hasPackageJson = fs.existsSync(path.join(skillPath, 'package.json'));
    const hasIndexJs = fs.existsSync(path.join(skillPath, 'index.js'));
    const hasDist = fs.existsSync(path.join(skillPath, 'dist/index.js'));

    let isBroken = false;

    if (!hasSkillMd) {
      report.missing_docs.push(skill.name);
      isBroken = true;
    }

    if (!hasPackageJson) {
      // Some skills might be just scripts, check for index.js
      if (!hasIndexJs && !hasDist) {
        report.missing_package.push(skill.name);
        isBroken = true;
      }
    }

    if (!isBroken) {
      report.healthy++;
    } else {
      report.broken.push(skill.name);
    }
  });

  console.log('\n📊 Skill Health Report:');
  console.log(`✅ Healthy: ${report.healthy}`);
  console.log(`⚠️  Broken/Incomplete: ${report.broken.length}`);
  console.log(`📚 Total: ${report.total}`);

  if (report.missing_docs.length > 0) {
    console.log('\n❌ Missing SKILL.md (Documentation):');
    report.missing_docs.forEach(s => console.log(` - ${s}`));
  }

  if (report.missing_package.length > 0) {
    console.log('\n❌ Missing package.json/index.js (Entrypoint):');
    report.missing_package.forEach(s => console.log(` - ${s}`));
  }

  // Write report to file for other agents to consume
  const reportFile = path.join(__dirname, 'health_report.json');
  fs.writeFileSync(reportFile, JSON.stringify(report, null, 2));
  console.log(`\n💾 Report saved to: ${reportFile}`);

  // Run Dependency Check
  console.log('\n--- Running Dependency Checks ---');
  dependencyCheck.checkDependencies();
}

checkSkills();
