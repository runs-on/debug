const childProcess = require('child_process')
const os = require('os')
const process = require('process')
const path = require('path')

const WINDOWS = 'win32'
const LINUX = 'linux'
const AMD64 = 'x64'
const ARM64 = 'arm64'

function chooseBinary() {
    const platform = os.platform()
    const arch = os.arch()

    if (platform === LINUX && arch === AMD64) {
        return 'runs-on-debug-linux-amd64'
    }
    if (platform === LINUX && arch === ARM64) {
        return 'runs-on-debug-linux-arm64'
    }
    if (platform === WINDOWS) {
        return 'runs-on-debug-windows-amd64.exe'
    }

    console.error(`Unsupported platform (${platform}) and architecture (${arch})`)
    process.exit(0)
}

function main() {
    const binary = chooseBinary()
    const mainScript = path.join(__dirname, binary)
    if (os.platform() === WINDOWS) {
        childProcess.execFileSync(mainScript, { stdio: 'inherit' })
        return
    }

    // Linux sources include root-owned journals and /var logs, so preserve the
    // workflow/AWS environment while starting the Go launcher through sudo.
    try {
        childProcess.execFileSync('sudo', ['-n', '-E', mainScript], { stdio: 'inherit' })
    } catch (error) {
        if (error.code === 'ENOENT') {
            childProcess.execFileSync(mainScript, { stdio: 'inherit' })
            return
        }
        throw error
    }
}

if (require.main === module) {
    main()
}
