# exec-ecs

`exec-ecs` is a command-line tool to simplify working with AWS ECS by streamlining the execution of commands on ECS tasks. This tool is designed for developers and operations teams to interact with ECS clusters and tasks efficiently.

---

## Features

- **Cluster Selection**: Easily select an ECS cluster to work with.
- **Service and Task Navigation**: Navigate through ECS services and tasks interactively.

---

## Installation

### Using the Install Script

To install `exec-ecs`, use the following command:

```bash
curl -fsSL https://raw.githubusercontent.com/DigitalTolk/exec-ecs/main/install.sh | bash
```

### Manual Installation

1. Go to the [Releases](https://github.com/DigitalTolk/exec-ecs/releases) page.
    
2. Download the appropriate binary for your platform:
    
    - **Linux**:
        - `exec-ecs_Linux_x86_64.tar.gz` (Linux 64-bit)
        - `exec-ecs_Linux_arm64.tar.gz` (Linux ARM64)
        - `exec-ecs_Linux_armv6.tar.gz` (Linux ARMv6)
        - `exec-ecs_Linux_i386.tar.gz` (Linux 32-bit)
    - **macOS**:
        - `exec-ecs_Darwin_x86_64.tar.gz` (macOS Intel)
        - `exec-ecs_Darwin_arm64.tar.gz` (macOS ARM)
    - **Windows**:
        - `exec-ecs_Windows_x86_64.zip` (Windows 64-bit)
        - `exec-ecs_Windows_arm64.zip` (Windows ARM64)
        - `exec-ecs_Windows_armv6.zip` (Windows ARMv6)
        - `exec-ecs_Windows_i386.zip` (Windows 32-bit)
3. Extract the file:
    
    - For `.tar.gz` files:
        
        `tar -xzf <filename>`
        
    - For `.zip` files (Windows): Use an unzip utility.
4. Place the `exec-ecs` binary in a directory in your `PATH`:
    
    - On Linux/macOS:
        
        `sudo mv exec-ecs /usr/local/bin/`
        
    - On Windows: Move the `exec-ecs.exe` file to a directory included in your `PATH` (e.g., `C:\Windows\System32`).
5. Verify installation:
    
    `exec-ecs --help`
