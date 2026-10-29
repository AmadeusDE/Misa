#!/usr/bin/env bash
# Cross-compile the APK's two native artifacts into android/app/src/main/jniLibs.
#
#   libmisa.so      CLI + TUI + ugh multicall (API 24), executed from nativeLibraryDir
#   libmisagui.so   c-shared GUI (API 26, -tags nough); the agent runs in the child
#
# ABIs: arm64-v8a and x86_64. The script is runnable on its own; Gradle calls it
# unless -PskipGoBuild is set, and then it refuses to package missing outputs.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

ver=$(tr -d '[:space:]' < android/ndk.version)
if [[ -z ${ANDROID_NDK_HOME:-} ]]; then
	for c in \
		"${ANDROID_SDK_ROOT:-}/ndk/$ver" \
		"${ANDROID_HOME:-}/ndk/$ver" \
		"$HOME/android-sdk/ndk/$ver" \
		"/opt/android-sdk/ndk/$ver" \
		"/usr/local/lib/android/sdk/ndk/$ver" \
		"/opt/android/android-ndk-r30"; do
		if [[ -d $c/toolchains/llvm/prebuilt ]]; then
			ANDROID_NDK_HOME=$c
			break
		fi
	done
fi
if [[ -z ${ANDROID_NDK_HOME:-} ]]; then
	echo "ANDROID_NDK_HOME is not set and NDK $ver was not found" >&2
	exit 1
fi

prebuilt="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt"
host=""
case "$(uname -s)-$(uname -m)" in
Linux-x86_64) host=linux-x86_64 ;;
Darwin-arm64) host=darwin-arm64 ;;
Darwin-x86_64) host=darwin-x86_64 ;;
esac
if [[ -z $host || ! -d $prebuilt/$host/bin ]]; then
	host=$(find "$prebuilt" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | head -n 1)
fi
bin="$prebuilt/$host/bin"
if [[ ! -x $bin/aarch64-linux-android24-clang || ! -x $bin/aarch64-linux-android26-clang ]]; then
	echo "NDK clang not found under $bin (pin $ver)" >&2
	exit 1
fi

epoch=$(git -C "$root" log -1 --format=%ct 2>/dev/null || echo 0)
export SOURCE_DATE_EPOCH=$epoch

mkdir -p android/app/src/main/assets
{
	printf '%s\n' misa ugh
	CGO_ENABLED=0 GOOS= GOARCH= go run ./android/listbuiltins
} | awk 'NF && !seen[$0]++' > android/app/src/main/assets/multicall.txt

ldflags='-s -w -buildid= -extldflags=-Wl,-z,max-page-size=16384'

build_one() {
	local abi=$1 goarch=$2 triple24=$3 triple26=$4
	local dest="android/app/src/main/jniLibs/$abi"
	mkdir -p "$dest"
	local cc24="$bin/$triple24" cc26="$bin/$triple26"
	if [[ ! -x $cc24 || ! -x $cc26 ]]; then
		echo "missing clang: $cc24 or $cc26" >&2
		exit 1
	fi
	echo "build $abi libmisa.so"
	GOOS=android GOARCH=$goarch CGO_ENABLED=1 CC=$cc24 \
		go build -buildvcs=false -trimpath -ldflags "$ldflags" \
		-o "$dest/libmisa.so" .
	echo "build $abi libmisagui.so"
	GOOS=android GOARCH=$goarch CGO_ENABLED=1 CC=$cc26 \
		go build -buildvcs=false -tags nough -buildmode=c-shared -trimpath -ldflags "$ldflags" \
		-o "$dest/libmisagui.so" ./cmd/misagui
	rm -f "$dest/libmisagui.h" "$dest"/libmisagui.so.h
	chmod +x "$dest/libmisa.so" "$dest/libmisagui.so"
	for so in "$dest/libmisa.so" "$dest/libmisagui.so"; do
		local aligns
		aligns=$(readelf -W -l "$so" | awk '/LOAD/ {print $NF}')
		if [[ -z $aligns ]]; then
			echo "no LOAD segment in $so" >&2
			exit 1
		fi
		local a
		for a in $aligns; do
			if [[ $a != 0x4000 ]]; then
				echo "$so LOAD align $a, want 0x4000" >&2
				readelf -W -l "$so" >&2
				exit 1
			fi
		done
	done
	# grep without -q reads the whole table. With pipefail, grep -q
	# closes the pipe early and nm dies with SIGPIPE (141), which looks
	# like a missing symbol.
	if ! nm -D "$dest/libmisagui.so" | grep -F JNI_OnLoad >/dev/null; then
		echo "$dest/libmisagui.so is missing JNI_OnLoad" >&2
		exit 1
	fi
	ls -lh "$dest/libmisa.so" "$dest/libmisagui.so"
}

build_one arm64-v8a arm64 aarch64-linux-android24-clang aarch64-linux-android26-clang
build_one x86_64 amd64 x86_64-linux-android24-clang x86_64-linux-android26-clang
echo "jniLibs ready"
