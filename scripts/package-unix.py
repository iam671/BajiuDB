"""Package an already-built Linux/macOS binary without running target code."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import plistlib
import re
import shutil
import stat
import struct
import tarfile
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parent.parent
MATERIALS = (
    'README.md', 'README.zh-CN.md', 'LICENSE', 'VALIDATION.md', 'design-qa.md',
    'docs', 'THIRD_PARTY_NOTICES.md', 'licenses', 'third_party_sources', 'tunnel',
)


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def validate_binary(path, target_os, arch):
    with path.open('rb') as stream:
        header = stream.read(64)
    if len(header) < 64:
        raise ValueError('Binary header is incomplete')
    if target_os == 'linux':
        if header[:6] != b'\x7fELF\x02\x01':
            raise ValueError('Expected a little-endian ELF64 binary')
        expected = {'amd64': 62, 'arm64': 183}[arch]
        actual = struct.unpack_from('<H', header, 18)[0]
    else:
        if header[:4] != b'\xcf\xfa\xed\xfe':
            raise ValueError('Expected a little-endian Mach-O 64-bit binary')
        expected = {'amd64': 0x01000007, 'arm64': 0x0100000c}[arch]
        actual = struct.unpack_from('<I', header, 4)[0]
    if actual != expected:
        raise ValueError('Binary architecture does not match the requested package')


def package(target_os, arch, binary):
    validate_binary(binary, target_os, arch)
    version = json.loads((ROOT / 'wails.json').read_text(encoding='utf-8'))['info']['productVersion']
    frontend = json.loads((ROOT / 'frontend/package.json').read_text(encoding='utf-8'))
    lock = json.loads((ROOT / 'frontend/package-lock.json').read_text(encoding='utf-8'))
    if not all(v == version for v in (frontend['version'], lock['version'], lock['packages']['']['version'])):
        raise ValueError('Product versions differ')
    if not isinstance(version, str) or re.fullmatch(r'\d+\.\d+\.\d+', version) is None:
        raise ValueError('Invalid product version')
    label = 'macos' if target_os == 'darwin' else 'linux'
    cpu = 'x64' if arch == 'amd64' else 'arm64'
    name = f'BajiuDB-{version}-{label}-{cpu}'
    dist = ROOT / 'dist'
    stage_root = ROOT / '.local' / 'unix-package-staging'
    if not dist.resolve().is_relative_to(ROOT) or not stage_root.resolve().is_relative_to(ROOT):
        raise ValueError('Output directories must stay inside the project')
    dist.mkdir(exist_ok=True)
    stage_root.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=name + '-', dir=stage_root) as temporary:
        stage = Path(temporary)
        release = stage / name
        release.mkdir()
        for material in MATERIALS:
            source = ROOT / material
            if source.is_dir():
                shutil.copytree(source, release / material)
            else:
                shutil.copyfile(source, release / material)
        if target_os == 'darwin':
            contents = release / 'Bajiu DB.app' / 'Contents'
            executable = contents / 'MacOS' / 'BajiuDB'
            resources = contents / 'Resources'
            executable.parent.mkdir(parents=True)
            resources.mkdir()
            metadata = {
                'CFBundleIdentifier': 'io.github.iam671.bajiudb',
                'CFBundleName': 'Bajiu DB', 'CFBundleDisplayName': 'Bajiu DB',
                'CFBundleExecutable': 'BajiuDB', 'CFBundlePackageType': 'APPL',
                'CFBundleShortVersionString': version, 'CFBundleVersion': version,
                'CFBundleIconFile': 'appicon.icns', 'LSMinimumSystemVersion': '12.0',
                'NSHighResolutionCapable': True,
            }
            (contents / 'Info.plist').write_bytes(plistlib.dumps(metadata))
            png = (ROOT / 'build/appicon.png').read_bytes()
            if png[:8] != b'\x89PNG\r\n\x1a\n' or struct.unpack('>II', png[16:24]) != (512, 512):
                raise ValueError('The app icon must be a 512x512 PNG')
            # ICNS ic09 stores the original 512px PNG without image modification.
            chunk = b'ic09' + struct.pack('>I', len(png) + 8) + png
            (resources / 'appicon.icns').write_bytes(b'icns' + struct.pack('>I', len(chunk) + 8) + chunk)
        else:
            executable = release / 'BajiuDB'
            shutil.copyfile(ROOT / 'build/appicon.png', release / 'appicon.png')
        shutil.copyfile(binary, executable)
        executable.chmod(0o755)
        relative_exe = executable.relative_to(release).as_posix()
        binary_hash = digest(binary)
        (release / 'SHA256SUMS.txt').write_text(f'{binary_hash}  {relative_exe}\n', encoding='utf-8')
        (release / 'BUILD-INFO.json').write_text(json.dumps({
            'product': 'Bajiu DB', 'version': version, 'os': target_os, 'arch': arch,
            'backend': 'Wails 3.1.0 experimental purego', 'cgoEnabled': False,
            'runtimeVerified': False, 'developerIDSigned': False, 'notarized': False,
            'binarySHA256': binary_hash,
        }, indent=2) + '\n', encoding='utf-8')
        if target_os == 'darwin':
            notes = ('# macOS build / macOS 构建\n\n'
                     f'Bajiu DB {version} · {cpu} · macOS 12+ deployment target\n\n'
                     'Extract the ZIP, then move Bajiu DB.app to Applications. Choose the build for your CPU. '
                     'This cross-compiled purego build has not been launched on a Mac. It has no Apple Developer ID signature or notarization. '
                     'macOS may block an unnotarized app; this package is for testing, not a signed installer.\n\n'
                     '解压后将 Bajiu DB.app 移入“应用程序”，按 CPU 选择版本。此包为交叉编译的 purego 测试构建，'
                     '尚未在 Mac 启动验证，没有 Apple Developer ID 签名和公证；macOS 可能阻止打开。\n')
        else:
            notes = ('# Linux build / Linux 构建\n\n'
                     f'Bajiu DB {version} · {cpu} · GTK4 / WebKitGTK 6.0\n\n'
                     'Extract the archive and run ./BajiuDB in a graphical desktop session. '
                     'GTK4 and WebKitGTK 6.0 runtime libraries are required and are not bundled. '
                     'On Ubuntu 24.04/Debian 13: sudo apt install libgtk-4-1 libwebkitgtk-6.0-4. '
                     'Remembered passwords additionally need an unlocked Secret Service (for example GNOME Keyring). '
                     'Other distributions need equivalent packages. Ubuntu 22.04/Debian 12 are not the target of this GTK4 package. '
                     'This cross-compiled experimental purego build has not been launched on Linux.\n\n'
                     '解压后在图形桌面运行 ./BajiuDB。需自行安装 GTK4、WebKitGTK 6.0，包内未附运行库；'
                     '记住密码还需已解锁的 Secret Service（如 GNOME Keyring）。此包尚未在 Linux 启动验证，'
                     '不以 Ubuntu 22.04/Debian 12 为目标。\n')
        (release / 'README-platform.md').write_text(notes, encoding='utf-8')
        suffix = '.zip' if target_os == 'darwin' else '.tar.gz'
        archive = stage / (name + suffix)
        if target_os == 'darwin':
            with zipfile.ZipFile(archive, 'w', zipfile.ZIP_DEFLATED, compresslevel=9) as output:
                for source in sorted(release.rglob('*')):
                    if not source.is_file():
                        continue
                    info = zipfile.ZipInfo(name + '/' + source.relative_to(release).as_posix())
                    info.create_system = 3
                    mode = 0o755 if source == executable else 0o644
                    info.external_attr = (stat.S_IFREG | mode) << 16
                    output.writestr(info, source.read_bytes(), compress_type=zipfile.ZIP_DEFLATED, compresslevel=9)
            with zipfile.ZipFile(archive) as output:
                member = name + '/' + relative_exe
                if hashlib.sha256(output.read(member)).hexdigest() != binary_hash:
                    raise ValueError('Packaged binary hash mismatch')
                if not (output.getinfo(member).external_attr >> 16) & 0o111:
                    raise ValueError('Packaged executable permission is missing')
        else:
            def normalize(info):
                info.uid = info.gid = 0
                info.uname = info.gname = ''
                info.mode = 0o755 if info.isdir() or info.name == name + '/' + relative_exe else 0o644
                return info
            with tarfile.open(archive, 'w:gz', compresslevel=9) as output:
                output.add(release, arcname=name, filter=normalize)
            with tarfile.open(archive, 'r:gz') as output:
                member = output.getmember(name + '/' + relative_exe)
                stream = output.extractfile(member)
                if stream is None or hashlib.file_digest(stream, 'sha256').hexdigest() != binary_hash:
                    raise ValueError('Packaged binary hash mismatch')
                if member.mode != 0o755:
                    raise ValueError('Packaged executable permission is missing')
        archive_hash = digest(archive)
        checksum = stage / (name + '.sha256')
        checksum.write_text(f'{archive_hash}  {name}{suffix}\n', encoding='utf-8')
        destination = (dist / name).resolve()
        # Resolve and bound every recursive replacement before touching old outputs.
        if destination.parent != dist.resolve() or not destination.name.startswith('BajiuDB-'):
            raise ValueError('Refusing to replace a path outside the release directory')
        if destination.exists():
            if destination.is_symlink() or not destination.is_dir():
                raise ValueError('Release directory is not a normal directory')
            shutil.rmtree(destination)
        shutil.move(str(release), destination)
        os.replace(archive, dist / archive.name)
        os.replace(checksum, dist / checksum.name)
        print(f'{name}{suffix}: {(dist / archive.name).stat().st_size:,} bytes; SHA-256 {archive_hash}')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--os', choices=('linux', 'darwin'), required=True)
    parser.add_argument('--arch', choices=('amd64', 'arm64'), required=True)
    parser.add_argument('--binary', type=Path, required=True)
    args = parser.parse_args()
    package(args.os, args.arch, args.binary.resolve(strict=True))
