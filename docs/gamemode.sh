#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

function enable() {
  # Enable debug logs
  # export PROTON_LOG=1
  # export DXVK_NVAPI_LOG_LEVEL=info
  # export DXVK_NVAPI_LOG_PATH=$HOME/logs
  # export DXVK_LOG_LEVEL=none
  # export NVPRESENT_LOG_LEVEL=4
  # export NVPRESENT_LOG_FILE=$HOME/logs/nv_smooth_motion.log

  # Enable CPU tweaks
  # sudo systemctl start scx.service
  # sudo cpupower frequency-set -g performance
  # sudo bash -c 'echo off > /sys/devices/system/cpu/smt/control'

  # Enable audio tweaks
  # export PIPEWIRE_LATENCY=1024/48000

  # Enable Nvidia tweaks
  export __GL_SHADER_DISK_CACHE=1
  export __GL_SHADER_DISK_CACHE_PATH="$HOME/.cache/nvidia"
  # export __GL_SHOW_GRAPHICS_OSD=1
  export __GL_THREADED_OPTIMIZATION=1
  # sudo nvidia-smi -gtt 88
  # sudo nvidia-settings --config="$HOME/.config/nvidia/settings" \
  #   -a "DigitalVibrance=200" \
  #   -a "GPUGraphicsClockOffset[3]=50" \
  #   -a "GPUMemoryTransferRateOffset[3]=500" \
  #   -a "GpuPowerMizerMode=1"

  # Enable Nvidia Smooth Motion
  # export NVPRESENT_ENABLE_SMOOTH_MOTION=1
  # Fix issues with third-party overlays
  # export NVPRESENT_QUEUE_FAMILY=1

  # Enable DXVK/Proton tweaks
  export DXVK_CONFIG_FILE="$HOME/.config/dxvk.conf"
  export DXVK_STATE_CACHE_PATH="$HOME/.cache/dxvk"
  export PROTON_ENABLE_WAYLAND=1
  export PROTON_ENABLE_HDR=1

  # Enable DLSS tweaks
  # export DXVK_NVAPI_SET_NGX_DEBUG_OPTIONS=DLSSIndicator=1024,DLSSGIndicator=2
  export DXVK_NVAPI_SET_NGX_DEBUG_OPTIONS=DLSSIndicator=0,DLSSGIndicator=0
  # This force the latest preset for SR, RR and framegen + updates the dlss via ngx
  export PROTON_ENABLE_NGX_UPDATER=1
  export DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE=on
  export DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE=on
  export DXVK_NVAPI_DRS_NGX_DLSS_FG_OVERRIDE=off
  export DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE_RENDER_PRESET_SELECTION=render_preset_latest
  export DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE_RENDER_PRESET_SELECTION=render_preset_latest

  # Enable Vulkan reflex compatibility layer
  # export VK_LAYER_DXVK_NVAPI_reflex=1
  # export DXVK_NVAPI_VKREFLEX=1
}

function disable() {
  # Disable debug logs
  export -n PROTON_LOG
  export -n DXVK_NVAPI_LOG_LEVEL

  # Disable CPU tweaks
  # sudo systemctl stop scx.service
  # sudo cpupower frequency-set -g powersave
  sudo bash -c 'echo on > /sys/devices/system/cpu/smt/control'

  # Disable audio tweaks
  export -n PIPEWIRE_LATENCY

  # Disable Nvidia tweaks
  export -n __GL_SHOW_GRAPHICS_OSD
  export -n __GL_THREADED_OPTIMIZATION
  # sudo nvidia-smi -gtt 85
  # sudo nvidia-settings --config="$HOME/.config/nvidia/settings" \
  #   -a "DigitalVibrance=200" \
  #   -a "GPUGraphicsClockOffset[3]=-200" \
  #   -a "GPUMemoryTransferRateOffset[3]=-2000" \
  #   -a "GpuPowerMizerMode=0"

  # Disable Wine/Proton tweaks
  export -n WINEESYNC
  export -n WINEFSYNC
  export -n WINEFSYNC_FUTEX2
  export -n WINEFSYNC_SPINCOUNT
  export -n WINE_FULLSCREEN_INTEGER_SCALING
  export -n WINE_LARGE_ADDRESS_AWARE
  export -n WINE_FULLSCREEN_FSR
  export -n WINE_FULLSCREEN_FSR_STRENGTH

  # Disable Nvidia Smooth Motion
  export -n NVPRESENT_ENABLE_SMOOTH_MOTION

  # Disable DXVK tweaks

  # Disable DLSS tweaks
  export -n PROTON_ENABLE_NGX_UPDATER
  export -n DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE
  export -n DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE
  export -n DXVK_NVAPI_DRS_NGX_DLSS_FG_OVERRIDE
  export -n DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE_RENDER_PRESET_SELECTION
  export -n DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE_RENDER_PRESET_SELECTION

  # Disable Vulkan reflex compatibility layer
  export -n VK_LAYER_DXVK_NVAPI_reflex
  export -n DXVK_NVAPI_VKREFLEX
}

trap 'disable &> /dev/null && echo "$(date +'%FT%T') Disabling gamemode..." | tee -a ~/logs/gamemode.log; exit' TERM EXIT ERR

case $1 in
  "--enable" | "-e" | "--mangohud")
    shift
    enable &> /dev/null \
      && echo "$(date +'%FT%T') Running: mangohud $*" | tee -a ~/logs/gamemode.log
    mangohud "${@}"
    ;;
  "--disable" | "-d")
    shift
    disable &> /dev/null \
      && echo "$(date +'%FT%T') Disabling gamemode..." | tee -a ~/logs/gamemode.log
    ;;
  "--no-mangohud" | "-nm")
    shift
    enable &> /dev/null \
      && echo "$(date +'%FT%T') Running: $*" | tee -a ~/logs/gamemode.log
    "${@}"
    ;;
  "--taskset" | "-ts")
    shift
    enable &> /dev/null \
      && echo "$(date +'%FT%T') Running: mangohud taskset -c 0-3,8-11 $*" | tee -a ~/logs/gamemode.log
    mangohud taskset -c 0-3,8-11 "${@}"
    ;;
  "--gamescope")
    shift
    enable &> /dev/null \
      && echo "$(date +'%FT%T') Running: gamescope mangohud $*" | tee -a ~/logs/gamemode.log
    gamescope --steam --fullscreen -- mangohud "${@}"
    ;;
  "--hdr")
    shift
    enable &> /dev/null \
      && echo "$(date +'%FT%T') Running: gamescope mangohud $*" | tee -a ~/logs/gamemode.log
    mangohud "${@}"
    ;;
  "--wait" | "-w")
    shift
    stty -echoctl
    enable &> /dev/null \
      && echo "$(date +'%FT%T') Enabling gamemode... (CTRL+C to exit)" \
      && read -r -d '' _ < /dev/tty
    ;;
  *)
    enable &> /dev/null \
      && echo "$(date +'%FT%T') Running: mangohud $*" | tee -a ~/logs/gamemode.log
    # ludusavi --config "$HOME"/.config/ludusavi wrap --gui --infer steam --
    mangohud "${@}"
    ;;
esac
