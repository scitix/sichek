#/bin/bash

mkdir -p /host-nvidia

# Determine which directory to use: prioritize /host-usr/lib/x86_64-linux-gnu, fallback to /host-usr/lib64
LIB_DIR=""
if [ "$(ls -A /host-usr/lib/x86_64-linux-gnu 2>/dev/null)" ]; then
LIB_DIR="/host-usr/lib/x86_64-linux-gnu"
elif [ "$(ls -A /host-usr/lib64 2>/dev/null)" ]; then
LIB_DIR="/host-usr/lib64"
else
echo "ERROR: Neither /host-usr/lib/x86_64-linux-gnu nor /host-usr/lib64 is a non-empty directory"
exit 1
fi

echo "Using library directory: $LIB_DIR"

# Create symlinks for libnvidia* libraries
for lib in `ls $LIB_DIR | grep libnvidia`;do
    if [ -f $LIB_DIR/$lib ]; then
    ln -sf $LIB_DIR/$lib /host-nvidia/$lib
    fi
done

# Create symlinks for libcuda* libraries
for lib in `ls $LIB_DIR | grep libcuda`;do
    if [ -f $LIB_DIR/$lib ]; then
    ln -sf $LIB_DIR/$lib /host-nvidia/$lib
    fi
done
echo "export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/host-nvidia" >> ~/.bashrc
