#!/usr/bin/env node
/**
 * SSH wrapper for git that uses the ssh2 Node.js library.
 * Usage: GIT_SSH_COMMAND="node /home/z/my-project/scripts/git-ssh-wrapper.cjs" git push
 */
const { Client } = require('ssh2');
const { spawn } = require('child_process');

const args = process.argv.slice(2);
let host = null;
let user = 'git';
let port = 22;
let command = null;

let i = 0;
while (i < args.length) {
  const a = args[i];
  if (a === '-o' || a === '-c' || a === '-i' || a === '-F' || a === '-l' || a === '-b' || a === '-p') {
    if (a === '-p') { i++; port = parseInt(args[i]) || 22; }
    else if (a === '-l') { i++; user = args[i]; }
    i++;
  } else if (a.startsWith('-')) { i++; }
  else if (a.includes('@')) { [user, host] = a.split('@'); i++; }
  else { command = a; i++; }
}

if (!host || !command) {
  console.error('git-ssh-wrapper: missing host or command', { args, host, command });
  process.exit(1);
}

// Clean the command
command = command.replace(/^['"]|['"]$/g, '');

const conn = new Client();
let stdInData = '';

conn.on('ready', () => {
  conn.exec(command, (err, stream) => {
    if (err) { console.error('exec error:', err); conn.end(); process.exit(1); }
    
    stream.on('data', (data) => process.stdout.write(data));
    stream.stderr.on('data', (data) => process.stderr.write(data));
    stream.on('close', (code) => { conn.end(); process.exit(code || 0); });
    
    // Forward stdin
    process.stdin.setRawMode(true);
    process.stdin.resume();
    process.stdin.on('data', (chunk) => {
      if (!stream.destroyed) stream.write(chunk);
    });
  });
});

// Try connecting with default keys
const homeDir = process.env.HOME || '/home/z';
const keyPaths = [
  `${homeDir}/.ssh/id_ed25519`,
  `${homeDir}/.ssh/id_rsa`,
  `${homeDir}/.ssh/id_ecdsa`,
];

// Check if any key files exist
const fs = require('fs');
const existingKeys = keyPaths.filter(p => fs.existsSync(p));

const opts = {
  host,
  port,
  username: user,
  readyTimeout: 10000,
};

if (existingKeys.length > 0) {
  // Load all existing keys
  opts.privateKey = fs.readFileSync(existingKeys[0]);
  if (existingKeys.length > 1) {
    opts.privateKeys = existingKeys.map(p => fs.readFileSync(p));
  }
} else {
  // Try ssh-agent if available
  opts.agent = process.env.SSH_AUTH_SOCK;
}

conn.connect(opts);
