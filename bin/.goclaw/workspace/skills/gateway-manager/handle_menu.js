const { exec } = require('child_process');
const fs = require('fs');
const path = require('path');
const os = require('os');

// --- Gateway Event Handler ---
// Handles 'application.bot.menu_v6' and 'card.action.trigger' events from Feishu Plugin.

/**
 * Intelligent Project Root Discovery
 */
function findProjectRoot(startDir) {
    let currentDir = startDir;
    while (true) {
        if (fs.existsSync(path.join(currentDir, 'openclaw.json')) || 
            fs.existsSync(path.join(currentDir, 'AGENTS.md'))) {
            return currentDir;
        }
        const parentDir = path.dirname(currentDir);
        if (parentDir === currentDir) {
            return path.resolve(__dirname, '../../');
        }
        currentDir = parentDir;
    }
}

const WORKSPACE_ROOT = findProjectRoot(__dirname);
const MEMORY_DIR = path.join(WORKSPACE_ROOT, 'memory');

const messageQueue = [];
let isProcessingQueue = false;

function processQueue() {
    if (isProcessingQueue || messageQueue.length === 0) return;
    isProcessingQueue = true;
    
    const task = messageQueue.shift();
    const { cmd, callback } = task;
    
    exec(cmd, { cwd: WORKSPACE_ROOT }, (error, stdout, stderr) => {
        if (error) {
            console.error(`[GatewayManager] Command failed: ${error.message}`);
        }
        if (callback) callback(error);
        
        isProcessingQueue = false;
        processQueue();
    });
}

function queueCommand(cmd, callback) {
    messageQueue.push({ cmd, callback });
    processQueue();
}

// Helper: Resolve a skill script path robustly
function getSkillScript(skillName, scriptName) {
    const possiblePaths = [
        path.join(WORKSPACE_ROOT, 'skills', skillName, scriptName),
        path.join(WORKSPACE_ROOT, 'extensions', skillName, scriptName),
        path.join(WORKSPACE_ROOT, 'skills', skillName, 'dist', scriptName)
    ];

    for (const p of possiblePaths) {
        if (fs.existsSync(p)) return p;
    }
    return null;
}

// Helper: Parse Master ID from USER.md
function getMasterId() {
    const defaultMaster = process.env.OPENCLAW_MASTER_ID; // Fallback
    try {
        const userMdPath = path.join(WORKSPACE_ROOT, 'USER.md');
        if (fs.existsSync(userMdPath)) {
            const content = fs.readFileSync(userMdPath, 'utf8');
            const match = content.match(/Owner \(Master\).*?Feishu ID:\s*`([^`]+)`/s);
            if (match && match[1]) {
                return match[1];
            }
        }
    } catch (e) {
        // Ignore
    }
    return defaultMaster;
}

function logAudit(action, userId, success, details = '') {
    const auditPath = path.join(MEMORY_DIR, 'admin_audit.log');
    const timestamp = new Date().toISOString();
    const line = `[${timestamp}] User:${userId} Action:${action} Success:${success} ${details}\n`;
    try {
        fs.appendFileSync(auditPath, line);
    } catch (e) {
        console.error(`[GatewayManager] Failed to write audit log: ${e.message}`);
    }
}

function sendFeedback(targetId, title, text, color = 'blue') {
    const scriptPath = getSkillScript('feishu-card', 'send.js');
    if (!scriptPath) return;

    const cmd = `node "${scriptPath}" --target "${targetId}" --title "${title}" --color "${color}"`;
    queueCommand(cmd);
}

function sendSticker(targetId, imagePath) {
    const scriptPath = getSkillScript('feishu-sticker', 'send.js');
    if (!scriptPath) {
        const cardScript = getSkillScript('feishu-card', 'send.js');
        if (cardScript) {
            const cmd = `node "${cardScript}" --target "${targetId}" --image-path "${imagePath}" --text "Meow! 😺"`;
            queueCommand(cmd);
        }
        return;
    }
    const cmd = `node "${scriptPath}" --target "${targetId}" --file "${imagePath}"`;
    queueCommand(cmd);
}

// --- Main Event Loop ---

let payload = '';
process.stdin.setEncoding('utf8');

process.stdin.on('data', chunk => { payload += chunk; });

process.stdin.on('end', () => {
    try {
        if (!payload.trim()) return;
        
        const data = JSON.parse(payload);
        
        // --- KEY EXTRACTION ---
        const eventKey = 
            data.event?.event_key || 
            data.event_key || 
            data.action?.value?.key || 
            data.action?.tag ||
            data.body?.event?.event_key;

        // --- ID EXTRACTION ---
        const userId = 
            data.operator?.operator_id?.open_id ||
            data.event?.operator?.operator_id?.open_id ||
            data.sender?.sender_id?.open_id ||
            data.header?.sender?.sender_id?.open_id ||
            data.event?.operator_id?.open_id ||
            data.open_id ||
            data.user_id ||
            data.operator_id?.open_id ||
            'unknown';

        // Log to memory file for Agent visibility (keep this for audit)
        const logPath = path.join(MEMORY_DIR, 'menu_events.json');
        if (!fs.existsSync(MEMORY_DIR)) fs.mkdirSync(MEMORY_DIR, { recursive: true });
        let logs = [];
        try { logs = JSON.parse(fs.readFileSync(logPath, 'utf8')); } catch(e) {}
        logs.push({ timestamp: new Date().toISOString(), eventKey, userId, raw: data });
        if (logs.length > 50) logs = logs.slice(-50);
        fs.writeFileSync(logPath, JSON.stringify(logs, null, 2));

        // Update Shared Context (Source-to-Destination Routing)
        try {
            const contextPath = path.join(MEMORY_DIR, 'context.json');
            let context = {};
            if (fs.existsSync(contextPath)) {
                try { context = JSON.parse(fs.readFileSync(contextPath, 'utf8')); } catch(e) {}
            }
            if (userId !== 'unknown') {
                context.last_active_user = userId;
                context.last_interaction_ts = Date.now();
                context.last_source = 'gateway-manager';
                fs.writeFileSync(contextPath, JSON.stringify(context, null, 2));
            }
        } catch (e) {
            console.error(`[GatewayManager] Failed to update context: ${e.message}`);
        }

        console.log(`[GatewayManager] Processing ${eventKey} from ${userId}`);

        // --- LOGIC ---
        const MASTER_ID = getMasterId();

        if (eventKey === 'restart_gateway') {
            if (userId !== MASTER_ID) {
                logAudit('restart_gateway', userId, false, 'Permission Denied');
                sendFeedback(userId, 'Permission Denied', '🚫 Only Master can restart the gateway.', 'red');
                return;
            }
            logAudit('restart_gateway', userId, true, 'Initiated');

            try {
                const ALERT_FILE = path.join(WORKSPACE_ROOT, 'memory/pending_restart_alert.json');
                fs.writeFileSync(ALERT_FILE, JSON.stringify({ userId: userId }));
            } catch (e) {
                console.error(`[GatewayManager] Failed to save alert file: ${e.message}`);
            }

            sendFeedback(userId, 'Gateway Manager', '🚀 Restarting...', 'orange');
            execSync('zeroclaw gateway restart', { stdio: 'inherit' });
        } 
        else if (eventKey === 'status_gateway') {
            exec('zeroclaw gateway status', { cwd: WORKSPACE_ROOT }, (err, stdout, stderr) => {
                const statusOutput = stdout || err?.message || 'Unknown';
                const color = statusOutput.includes('Active: active') ? 'green' : 'red';
                sendFeedback(userId, 'Gateway Status', `\`\`\`\n${statusOutput.trim()}\n\`\`\``, color);
            });
        } 
        else if (eventKey === 'system_info') {
             try {
                const memUsage = process.memoryUsage();
                const totalMem = os.totalmem();
                const freeMem = os.freemem();
                const usedMem = totalMem - freeMem;
                
                const uptimeSec = os.uptime();
                const d = Math.floor(uptimeSec / (3600*24));
                const h = Math.floor(uptimeSec % (3600*24) / 3600);
                const m = Math.floor(uptimeSec % 3600 / 60);
                const uptimeStr = (d > 0 ? `${d}d ` : '') + (h > 0 ? `${h}h ` : '') + `${m}m`;

                const info = `
**System Info**:
- **Node**: ${process.version}
- **OS**: ${os.type()} ${os.release()}
- **Memory**: ${Math.round(usedMem / 1024 / 1024)}MB / ${Math.round(totalMem / 1024 / 1024)}MB
- **RSS**: ${Math.round(memUsage.rss / 1024 / 1024)}MB
- **Uptime**: ${uptimeStr}
- **Load**: ${os.loadavg().map(l => l.toFixed(2)).join(', ')}
                `.trim();
                sendFeedback(userId, 'System Info', info, 'blue');
             } catch (err) {
                 sendFeedback(userId, 'System Info', `Error: ${err.message}`, 'red');
             }
        }
        else if (eventKey === 'test_menu_button') {
            console.log('[GatewayManager] Action: CUTE MODE');
            const stickersDir = path.join(WORKSPACE_ROOT, 'media/stickers');
            let stickerPath = '';
            
            if (fs.existsSync(stickersDir)) {
                const files = fs.readdirSync(stickersDir).filter(f => f.match(/\.(jpg|jpeg|png|webp)$/i));
                if (files.length > 0) {
                    const cuteOnes = files.filter(f => f.includes('salute') || f.includes('cute'));
                    const targetFiles = cuteOnes.length > 0 ? cuteOnes : files;
                    const randomFile = targetFiles[Math.floor(Math.random() * targetFiles.length)];
                    stickerPath = path.join(stickersDir, randomFile);
                }
            }
            
            if (stickerPath && userId !== 'unknown') {
                sendSticker(userId, stickerPath);
                sendFeedback(userId, '系统提示', '卖萌成功！ 😺❤️', 'violet');
            } else {
                if (userId !== 'unknown') sendFeedback(userId, 'Bot', 'No stickers found 😿', 'grey');
            }
        } 
    } catch (e) {
        console.error(`[GatewayManager] Error: ${e.message}`);
    }
});
