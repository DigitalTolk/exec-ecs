# exec-ecs

`exec-ecs` is a command-line tool to simplify working with AWS ECS by streamlining the execution of commands on ECS tasks. This tool is designed for developers and operations teams to interact with ECS clusters and tasks efficiently.

---

## Features

- **Cluster Selection**: Easily select an ECS cluster to work with.
- **Service and Task Navigation**: Navigate through ECS services and tasks interactively.

---

# Installation

The `exec-ecs` tool can be installed easily on macOS, Linux, and Windows. Follow the appropriate instructions below based on your operating system.

---

## macOS and Linux Installation

To install `exec-ecs` on macOS or Linux, use the following command:

```bash
curl -fsSL https://raw.githubusercontent.com/DigitalTolk/exec-ecs/main/install.sh | bash
```

### Installing a Specific Version

If you'd like to install a specific version, use the `--version` flag with the install script:

```bash
curl -fsSL https://raw.githubusercontent.com/DigitalTolk/exec-ecs/main/install.sh | bash -s -- --version 1.1.3
```

Replace `1.1.3` with the desired version number. If no version is specified, the latest version will be installed by default.

---

## Windows Installation

Windows users should use the provided batch script.

### Steps:

1. Download the installation script:
   - Visit <https://raw.githubusercontent.com/DigitalTolk/exec-ecs/main/install-exec-ecs.bat>.
   - Save the file as `install-exec-ecs.bat`.

2. Run the script:
   - Right-click the `install-exec-ecs.bat` file and select **Run as Administrator**.
   - To install the latest version, simply run the script without any arguments:
     ```bash
     install-exec-ecs.bat
     ```

3. To install a specific version:
   - Run the script with the desired version number:
     ```bash
     install-exec-ecs.bat 1.1.3
     ```
   - Replace `1.1.3` with the version you wish to install.

4. The script will:
   - Download the `exec-ecs` executable for Windows.
   - Extract the binary.
   - Place it in `C:\Program Files\exec-ecs`.
   - Add it to your system's PATH.

---

## Verifying Installation

After installation, verify that `exec-ecs` is installed correctly by checking its version:

```bash
exec-ecs --version
```

On Windows, you may need to restart your terminal or Command Prompt for the changes to take effect.

---

## Uninstallation

To uninstall `exec-ecs`, simply remove the binary:

- **Linux/macOS**: Delete the binary from `/usr/local/bin`:
  ```bash
  sudo rm /usr/local/bin/exec-ecs
  ```
- **Windows**: Delete the `exec-ecs.exe` file from `C:\Program Files\exec-ecs`.

---


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
