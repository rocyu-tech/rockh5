module.exports = {
  apps: [
    {
      name: "rockh5",
      script: "npx",
      args: "next dev -p 8890",
      instances: 1,
      exec_mode: "fork",
      watch: true,
      watch_delay: 1000
    }
  ]
};
