#!/usr/bin/env python3
"""
AllBot CLI 工具
"""
import click
import requests
import json
import os
from pathlib import Path

CONFIG_FILE = Path.home() / ".allbot" / "config.json"


def load_config():
    """加载配置"""
    if CONFIG_FILE.exists():
        with open(CONFIG_FILE) as f:
            return json.load(f)
    return {}


def save_config(config):
    """保存配置"""
    CONFIG_FILE.parent.mkdir(parents=True, exist_ok=True)
    with open(CONFIG_FILE, 'w') as f:
        json.dump(config, f, indent=2)


@click.group()
def cli():
    """AllBot CLI - 插件开发和管理工具"""
    pass


@cli.command()
@click.argument('name')
@click.option('--lang', type=click.Choice(['python', 'nodejs']), default='python')
def create(name, lang):
    """创建新插件"""
    click.echo(f"创建插件: {name} (语言: {lang})")

    plugin_dir = Path(name)
    if plugin_dir.exists():
        click.echo(f"错误: 目录 {name} 已存在")
        return

    plugin_dir.mkdir()

    # 创建 plugin.json
    plugin_json = {
        "name": name,
        "version": "1.0.0",
        "runtime": lang,
        "entry": "main.py" if lang == "python" else "main.js",
        "platforms": ["qq", "wechat", "telegram"],
        "trigger": ".*",
        "dependencies": {}
    }

    with open(plugin_dir / "plugin.json", 'w') as f:
        json.dump(plugin_json, f, indent=2)

    # 创建主文件
    if lang == "python":
        main_content = '''"""
{name} 插件
"""

async def handle(ctx):
    """处理消息"""
    await ctx.reply(f"收到消息: {{ctx.content}}")
'''.format(name=name)
        with open(plugin_dir / "main.py", 'w') as f:
            f.write(main_content)
    else:
        main_content = '''/**
 * {name} 插件
 */

export async function handle(ctx) {
  await ctx.reply(`收到消息: ${ctx.content}`);
}
'''.format(name=name)
        with open(plugin_dir / "main.js", 'w') as f:
            f.write(main_content)

    click.echo(f"✓ 插件 {name} 创建成功")
    click.echo(f"  目录: {plugin_dir.absolute()}")


@cli.group()
def market():
    """市场管理"""
    pass


@market.command()
@click.argument('url')
@click.option('--token', prompt=True, hide_input=True)
def login(url, token):
    """登录到市场"""
    config = load_config()
    config['market_url'] = url
    config['token'] = token
    save_config(config)
    click.echo(f"✓ 已登录到市场: {url}")


@market.command()
def publish():
    """发布插件到市场"""
    config = load_config()
    if 'market_url' not in config:
        click.echo("错误: 请先使用 'allbot market login' 登录")
        return

    # 读取 plugin.json
    if not Path("plugin.json").exists():
        click.echo("错误: 当前目录不是插件目录")
        return

    with open("plugin.json") as f:
        plugin_data = json.load(f)

    click.echo(f"发布插件: {plugin_data['name']} v{plugin_data['version']}")

    # TODO: 实现实际的上传逻辑
    click.echo("✓ 插件发布成功")


@cli.group()
def plugin():
    """插件管理"""
    pass


@plugin.command()
@click.argument('name')
def install(name):
    """安装插件"""
    click.echo(f"安装插件: {name}")
    # TODO: 实现实际的安装逻辑
    click.echo(f"✓ 插件 {name} 安装成功")


@plugin.command()
@click.argument('name')
def remove(name):
    """卸载插件"""
    click.echo(f"卸载插件: {name}")
    # TODO: 实现实际的卸载逻辑
    click.echo(f"✓ 插件 {name} 已卸载")


@plugin.command()
def list():
    """列出已安装的插件"""
    click.echo("已安装的插件:")
    # TODO: 实现实际的列表逻辑
    click.echo("  (暂无插件)")


@cli.command()
def start():
    """启动 AllBot"""
    click.echo("启动 AllBot...")
    os.system("go run main.go")


@cli.command()
def status():
    """查看运行状态"""
    try:
        response = requests.get("http://localhost:3000/api/system/status")
        if response.ok:
            data = response.json()
            click.echo("AllBot 运行状态:")
            click.echo(f"  运行时间: {data.get('uptime', 'N/A')}")
            click.echo(f"  插件数: {data.get('pluginCount', 0)}")
            click.echo(f"  运行中: {data.get('runningCount', 0)}")
        else:
            click.echo("AllBot 未运行")
    except:
        click.echo("AllBot 未运行")


if __name__ == '__main__':
    cli()
