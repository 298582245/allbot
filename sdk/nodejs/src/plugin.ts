/**
 * 插件启动和 gRPC 服务
 */

export function startPlugin() {
  const args = process.argv.slice(2);
  const portArg = args.find(arg => arg.startsWith('--port='));
  const port = portArg ? parseInt(portArg.split('=')[1]) : 50051;

  const pluginId = process.env.ALLBOT_PLUGIN_ID || 'unknown';

  console.log(`Starting plugin: ${pluginId} on port ${port}`);

  // 动态导入插件的 main.js
  const mainPath = require('path').join(process.cwd(), 'main.js');

  try {
    const pluginModule = require(mainPath);

    if (!pluginModule.handle) {
      console.error('Error: handle function not found in main.js');
      process.exit(1);
    }

    const handleFunc = pluginModule.handle;

    // TODO: 启动 gRPC 服务器
    console.log(`Plugin ${pluginId} started successfully on port ${port}`);

    // 保持进程运行
    process.on('SIGINT', () => {
      console.log(`Plugin ${pluginId} shutting down...`);
      process.exit(0);
    });

  } catch (error) {
    console.error(`Failed to load plugin: ${error}`);
    process.exit(1);
  }
}

if (require.main === module) {
  startPlugin();
}
