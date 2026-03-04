const nodemailer = require('nodemailer');

// 默认配置 - 使用 126 邮箱
const DEFAULT_SMTP_CONFIG = {
    host: process.env.SMTP_HOST || 'smtp.126.com',
    port: parseInt(process.env.SMTP_PORT || '465'),
    secure: true,
    auth: {
        user: process.env.SMTP_USER || 'lj7788@126.com',
        pass: process.env.SMTP_PASS || 'KYT9pthptRut742V'
    }
};

async function sendEmail(to, subject, text, html = null, smtpConfig = null) {
    try {
        const config = smtpConfig || DEFAULT_SMTP_CONFIG;
        
        const transporter = nodemailer.createTransport({
            host: config.host,
            port: config.port,
            secure: config.secure,
            auth: config.auth
        });

        const mailOptions = {
            from: config.auth.user,
            to: to,
            subject: subject,
            text: text
        };

        if (html) {
            mailOptions.html = html;
        }

        const info = await transporter.sendMail(mailOptions);
        const successMsg = `✅ 邮件发送成功！\n📧 收件人: ${to}\n📋 主题: ${subject}\n🆔 消息ID: ${info.messageId}`;
        console.log(successMsg);
        return { success: true, messageId: info.messageId };
    } catch (error) {
        console.error('Error sending email:', error.message);
        return { success: false, error: error.message };
    }
}

if (require.main === module) {
    const args = process.argv.slice(2);
    let to = '';
    let subject = '';
    let body = '';
    let recipient = '';
    let content = '';

    for (let i = 0; i < args.length; i++) {
        if (args[i] === '--to' && args[i + 1]) {
            to = args[i + 1];
        }
        if (args[i] === '--subject' && args[i + 1]) {
            subject = args[i + 1];
        }
        if (args[i] === '--body' && args[i + 1]) {
            body = args[i + 1];
        }
        if (args[i] === '--recipient' && args[i + 1]) {
            recipient = args[i + 1];
        }
        if (args[i] === '--content' && args[i + 1]) {
            content = args[i + 1];
        }
    }

    if (!to && !recipient && !process.env.EMAIL_DATA) {
        process.stdin.setEncoding('utf8');
        let inputData = '';
        
        process.stdin.on('data', (chunk) => {
            inputData += chunk.toString();
        });
        
        process.stdin.on('end', () => {
            try {
                const data = JSON.parse(inputData);
                if (data.to) to = data.to;
                if (data.recipient) to = data.recipient;
                if (data.subject) subject = data.subject;
                if (data.body) body = data.body;
                if (data.content) body = data.content;
                
                to = to || recipient;
                body = body || content;
                
                if (!to || !subject || !body) {
                    console.log('Error: Missing required parameters');
                    process.exit(1);
                }
                
                sendEmail(to, subject, body).then(result => {
                    console.log(JSON.stringify(result));
                    process.exit(result.success ? 0 : 1);
                }).catch(err => {
                    console.error(JSON.stringify({ success: false, error: err.message }));
                    process.exit(1);
                });
            } catch (e) {
                console.error(JSON.stringify({ success: false, error: 'JSON parse error: ' + e.message }));
                process.exit(1);
            }
        });
    } else if (process.env.EMAIL_DATA) {
        try {
            const data = JSON.parse(process.env.EMAIL_DATA);
            if (data.to) to = data.to;
            if (data.recipient) to = data.recipient;
            if (data.subject) subject = data.subject;
            if (data.body) body = data.body;
            if (data.content) body = data.content;
            
            to = to || recipient;
            body = body || content;
            
            if (!to || !subject || !body) {
                console.log('Error: Missing required parameters');
                process.exit(1);
            }
            
            sendEmail(to, subject, body).then(result => {
                console.log(JSON.stringify(result));
                process.exit(result.success ? 0 : 1);
            }).catch(err => {
                console.error(JSON.stringify({ success: false, error: err.message }));
                process.exit(1);
            });
        } catch (e) {
            console.error(JSON.stringify({ success: false, error: 'JSON parse error: ' + e.message }));
            process.exit(1);
        }
    } else {
        to = to || recipient;
        body = body || content;
        
        if (!to || !subject || !body) {
            console.log('Error: Missing required parameters');
            process.exit(1);
        }
        
        sendEmail(to, subject, body).then(result => {
            console.log(JSON.stringify(result));
            process.exit(result.success ? 0 : 1);
        }).catch(err => {
            console.error(JSON.stringify({ success: false, error: err.message }));
            process.exit(1);
        });
    }
}

module.exports = { sendEmail };
