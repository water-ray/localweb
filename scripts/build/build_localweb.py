#!/usr/bin/env python3
"""
LocalWeb 跨平台编译脚本
支持编译 Windows (amd64) 和 Linux (amd64) 版本
"""

import os
import sys
import shutil
import subprocess
from pathlib import Path


def run_command(cmd, env=None, cwd=None):
    """执行命令并返回是否成功"""
    print(f"\n➤ {' '.join(cmd)}")
    try:
        process = subprocess.run(
            cmd,
            env=env or os.environ.copy(),
            cwd=cwd,
            capture_output=False,
            text=True
        )
        return process.returncode == 0
    except FileNotFoundError as e:
        print(f"✗ 命令未找到: {e}")
        return False


def build_localweb(goos, goarch, output_dir, config_file):
    """编译 LocalWeb 指定平台版本"""
    
    # 设置环境变量
    build_env = os.environ.copy()
    build_env["GOOS"] = goos
    build_env["GOARCH"] = goarch
    build_env["CGO_ENABLED"] = "0"
    
    # 确定输出文件名
    if goos == "windows":
        output_name = "localweb.exe"
    else:
        output_name = "localweb"
    
    output_path = os.path.join(output_dir, output_name)
    
    print(f"\n{'='*60}")
    print(f"编译平台: {goos}/{goarch}")
    print(f"输出文件: {output_path}")
    print(f"{'='*60}")
    
    # 执行 go build
    build_cmd = [
        "go",
        "build",
        "-o",
        output_path,
        "./cmd/localweb"
    ]
    
    if not run_command(build_cmd, env=build_env):
        print(f"✗ 编译失败: {goos}/{goarch}")
        return False
    
    print(f"✓ 编译成功: {output_path}")
    
    # 复制配置文件
    if os.path.exists(config_file):
        dest_config = os.path.join(output_dir, "config.example.json")
        shutil.copy2(config_file, dest_config)
        print(f"✓ 复制配置: {dest_config}")
    
    return True


def main():
    """主函数"""
    
    # 获取工作目录
    if len(sys.argv) > 1:
        work_dir = sys.argv[1]
    else:
        work_dir = os.getcwd()
    
    # 构建路径
    config_file = os.path.join(work_dir, "config.example.json")
    bin_base = os.path.join(work_dir, "Bin")
    
    # 清理并创建输出目录
    platforms = [
        ("windows", "amd64"),
        ("linux", "amd64"),
    ]
    
    all_success = True
    
    for goos, goarch in platforms:
        # 为每个平台创建独立目录
        platform_name = f"localweb-{goos}-{goarch}"
        output_dir = os.path.join(bin_base, platform_name)
        
        # 清理旧目录
        if os.path.exists(output_dir):
            print(f"\n清理旧目录: {output_dir}")
            shutil.rmtree(output_dir)
        
        # 创建新目录
        os.makedirs(output_dir, exist_ok=True)
        print(f"创建输出目录: {output_dir}")
        
        # 编译
        if not build_localweb(goos, goarch, output_dir, config_file):
            all_success = False
    
    print(f"\n{'='*60}")
    if all_success:
        print("✓ 所有平台编译完成")
        print(f"\n编译输出:")
        for goos, goarch in platforms:
            platform_name = f"localweb-{goos}-{goarch}"
            output_dir = os.path.join(bin_base, platform_name)
            print(f"  • {platform_name}/")
            for item in os.listdir(output_dir):
                print(f"    - {item}")
        sys.exit(0)
    else:
        print("✗ 部分平台编译失败")
        sys.exit(1)


if __name__ == "__main__":
    main()
