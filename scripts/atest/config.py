#!/usr/bin/env python3
"""
Sichek Configuration Management Tool
"""

import os
import sys
from pathlib import Path
from typing import Dict, Optional

try:
    import yaml
except ImportError:
    print("pyyaml not installed, installing...")
    import subprocess
    subprocess.check_call([sys.executable, "-m", "pip", "install", "-q", "pyyaml"])
    import yaml

CONFIG_KEYS = [
    "image_repo",
    "image_tag",
    "pytorchjob_image_repo",
    "pytorchjob_image_tag",
    "scheduler",
    "pytorchjob_host_dir",
    "roce_shared_mode",
    "default_spec",
    "swanlab_api_key",
    "swanlab_workspace",
    "swanlab_project",
    "swanlab_experiment_name",
]

CONFIG_DIR = Path.home() / ".sichek"
CONFIG_FILE = CONFIG_DIR / "config.yaml"


def get_config_path() -> Path:
    return CONFIG_FILE


def load_config() -> Dict[str, str]:
    config = {}
    if CONFIG_FILE.exists():
        try:
            with open(CONFIG_FILE, 'r') as f:
                config = yaml.safe_load(f) or {}
        except Exception as e:
            print(f"Warning: Failed to load config file: {e}")
    return config


def save_config(config: Dict[str, str]) -> bool:
    try:
        CONFIG_DIR.mkdir(parents=True, exist_ok=True)
        with open(CONFIG_FILE, 'w') as f:
            yaml.dump(config, f, default_flow_style=False, sort_keys=False)
        return True
    except Exception as e:
        print(f"Error: Failed to save config file: {e}")
        return False


def ask(prompt: str, default: str = "") -> str:
    if default:
        full_prompt = f"{prompt} [{default}]: "
    else:
        full_prompt = f"{prompt}: "
    
    try:
        user_input = input(full_prompt).strip()
        return user_input if user_input else default
    except (EOFError, KeyboardInterrupt):
        print("\nCancelled.")
        sys.exit(0)


def validate_spec_exists(spec_name: str) -> bool:
    if not spec_name:
        return True
    
    default_production_path = Path("/var/sichek/config")
    spec_path = default_production_path / spec_name
    
    if spec_path.exists():
        print(f"Found spec file in production path: {spec_path}")
        return True
    
    if spec_name.startswith("http://") or spec_name.startswith("https://"):
        print(f"Spec file is a URL: {spec_name}")
        return True
    
    spec_url = os.getenv("SICHEK_SPEC_URL")
    if spec_url:
        print(f"Spec file '{spec_name}' not found locally.")
        print(f"   Will try to download from SICHEK_SPEC_URL: {spec_url}")
        return True
    
    print("Spec validation failed!")
    print(f"Spec file '{spec_name}' not found in:")
    print(f"   - Production path: {default_production_path}")
    if spec_url:
        print(f"   - SICHEK_SPEC_URL: {spec_url}")
    else:
        print("   - SICHEK_SPEC_URL: (SICHEK_SPEC_URL environment variable not set)")
    print()
    print("Please ensure the spec file exists or set SICHEK_SPEC_URL")
    print()
    return False


def config_create():
    config = load_config()
    
    print("🔧 Sichek Configuration Setup")
    print("=" * 50)
    print()
    
    new_config = {}
    new_config["image_repo"] = ask(
        "sichek image repository",
        config.get("image_repo", "ghcr.io/scitix/sichek")
    )
    new_config["image_tag"] = ask(
        "sichek image tag",
        config.get("image_tag", "latest")
    )
    new_config["pytorchjob_image_repo"] = ask(
        "pytorchjob image repository",
        config.get("pytorchjob_image_repo", "")
    )
    new_config["pytorchjob_image_tag"] = ask(
        "pytorchjob image tag",
        config.get("pytorchjob_image_tag", "")
    )
    new_config["scheduler"] = ask(
        "k8s scheduler[si-scheduler, default-scheduler]",
        config.get("scheduler", "si-scheduler")
    )
    new_config["pytorchjob_host_dir"] = ask(
        "pytorchjob host directory to mount",
        config.get("pytorchjob_host_dir", "")
    )
    new_config["roce_shared_mode"] = ask(
        "roce shared mode[none, macvlan, volcengine, lingjun]",
        config.get("roce_shared_mode", "none")
    )
    new_config["default_spec"] = ask(
        "default spec",
        config.get("default_spec", "default_spec.yaml")
    )
    new_config["swanlab_api_key"] = ask(
        "swanlab api key",
        config.get("swanlab_api_key", "")
    )
    new_config["swanlab_workspace"] = ask(
        "swanlab workspace",
        config.get("swanlab_workspace", "")
    )
    new_config["swanlab_proj_name"] = ask(
        "swanlab project",
        config.get("swanlab_proj_name", "")
    )
    
    # Validate default_spec if provided
    if new_config.get("default_spec"):
        if not validate_spec_exists(new_config["default_spec"]):
            print("Spec validation failed, but continuing...")
    
    # Merge with existing config
    config.update(new_config)
    
    # Save config
    if save_config(config):
        print(f"Config saved to {CONFIG_FILE}")
    else:
        print("Failed to save config")
        sys.exit(1)


def config_view():
    config = load_config()
    
    if not config:
        print("No configuration found. Run 'config create' first.")
        return
    
    print("Current configuration:")
    for key in CONFIG_KEYS:
        val = config.get(key, "")
        print(f"  {key:<30} : {val}")


def config_set():
    config = load_config()
    
    if not config:
        print("No configuration found. Run 'config create' first.")
        return
    
    updated_keys = []
    
    while True:
        print("\n" + "=" * 60)
        print("Choose the key to modify:")
        for i, key in enumerate(CONFIG_KEYS, 1):
            val = config.get(key, "")
            # Show indicator if this key was already updated in this session
            updated_marker = " [updated]" if key in updated_keys else ""
            print(f"  [{i}] {key:<30} (current: {val}){updated_marker}")
        
        print(f"\n  [0] Done (save and exit)")
        if updated_keys:
            print(f"  Updated keys in this session: {', '.join(updated_keys)}")
        
        try:
            choice = input("\nEnter number to select key (or 0 to finish): ").strip()
            
            if not choice:
                continue
            
            idx = int(choice)
            
            if idx == 0:
                # User wants to finish
                if updated_keys:
                    if save_config(config):
                        print(f"\nSuccessfully saved {len(updated_keys)} key(s): {', '.join(updated_keys)}")
                    else:
                        print("\nFailed to save config")
                else:
                    print("\nNo changes made.")
                break
            
            if idx < 1 or idx > len(CONFIG_KEYS):
                print("Invalid choice. Please enter a number between 1 and {} or 0 to finish.".format(len(CONFIG_KEYS)))
                continue
            
            selected_key = CONFIG_KEYS[idx - 1]
            current_val = config.get(selected_key, "")
            
            prompt = f"Enter new value for '{selected_key}'"
            if current_val:
                prompt += f" [current: {current_val}]"
            prompt += " (press Enter to skip): "
            
            new_val = input(prompt).strip()
            
            if not new_val:
                print("Skipped (no changes made to this key).")
                continue
            
            config[selected_key] = new_val
            if selected_key not in updated_keys:
                updated_keys.append(selected_key)
            
            print(f"Updated {selected_key} = {new_val}")
            
            # Ask if user wants to continue
            continue_choice = input("\nContinue setting other keys? [Y/n]: ").strip().lower()
            if continue_choice in ('n', 'no'):
                # Save and exit
                if updated_keys:
                    if save_config(config):
                        print(f"\nSuccessfully saved {len(updated_keys)} key(s): {', '.join(updated_keys)}")
                    else:
                        print("\nFailed to save config")
                break
            
        except ValueError:
            print("Invalid input. Please enter a number.")
            continue
        except (KeyboardInterrupt, EOFError):
            print("\n\nCancelled.")
            if updated_keys:
                save_choice = input(f"\nSave {len(updated_keys)} updated key(s) before exiting? [Y/n]: ").strip().lower()
                if save_choice not in ('n', 'no'):
                    if save_config(config):
                        print(f"Saved {len(updated_keys)} key(s): {', '.join(updated_keys)}")
            break


def main():
    if len(sys.argv) < 2:
        print("Usage: config.py {create|view|set}")
        sys.exit(1)
    
    command = sys.argv[1]
    
    if command == "create":
        config_create()
    elif command == "view":
        config_view()
    elif command == "set":
        config_set()
    else:
        print(f"Unknown command: {command}")
        print("Usage: config.py {create|view|set}")
        sys.exit(1)


if __name__ == "__main__":
    main()
