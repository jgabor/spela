# DRS Settings

DXVK-NVAPI newer than Commit [c13ab33](https://github.com/jp7677/dxvk-nvapi/commit/c13ab338cd8d2e890fd69e695f5f830e345e2f37) allows to pass driver settings to an application; when requested by an application. Those driver settings are set with environment variables.

`DXVK_NVAPI_DRS_SETTINGS` allows providing custom values for arbitrary driver settings with keys and values of DWORD (u32) types. Format is `setting1=value1,setting2=value2,…` where both `settingN` and `valueN` are unsigned 32-bit integers and can be prefixed with `0x` or `0X` to parse them as hexadecimal numbers. Additionally, several well known settings are supported in their named/text form, for both keys and values. Each supported named setting can also be configured with a dedicated environment variable. The name of said environment variable is case-sensitive, the value is not. If a setting is set via both `DXVK_NVAPI_DRS_SETTINGS` and its own dedicated environment variable, the latter takes precedence.

## Well known settings

### Override DLSS-SR mode to be DLAA

- Key: `NGX_DLAA_OVERRIDE`
- Values:
  - `DLAA_DEFAULT`
  - `DLAA_ON`
  - `DEFAULT`

Example 1: `DXVK_NVAPI_DRS_SETTINGS=ngx_dlaa_override=dlaa_on`

Example 2: `DXVK_NVAPI_DRS_NGX_DLAA_OVERRIDE=dlaa_on`

### Override DLSS-SR performance mode to be ultra-perfomance

- Key: `NGX_DLSS_OVERRIDE_OPTIMAL_SETTINGS`
- Values:
  - `NONE`
  - `PERF_TO_9X`
  - `DEFAULT`

Example 1: `DXVK_NVAPI_DRS_SETTINGS=ngx_dlss_override_optimal_settings=perf_to_9x`

Example 2: `DXVK_NVAPI_DRS_NGX_DLSS_OVERRIDE_OPTIMAL_SETTINGS=perf_to_9x`

### Override DLSS-SR performance mode

- Key: `NGX_DLSS_SR_MODE`
- Values:
  - `PERFORMANCE`
  - `BALANCED`
  - `QUALITY`
  - `SNIPPET_CONTROLLED`
  - `DLAA`
  - `ULTRA_PERFORMANCE`
  - `CUSTOM`
  - `DEFAULT`

Example 1: `DXVK_NVAPI_DRS_SETTINGS=ngx_dlss_sr_mode=dlaa`

Example 2: `DXVK_NVAPI_DRS_NGX_DLSS_SR_MODE=dlaa`

Use [Override DLSS-SR scaling ratio](https://github.com/jp7677/dxvk-nvapi/wiki/Passing-driver-settings#override-dlss-sr-scaling-ratio) for specifying `CUSTOM` scales. Setting a custom scale of 75% can be achieved with `DXVK_NVAPI_DRS_NGX_DLSS_RR_MODE=custom,NGX_DLSS_RR_OVERRIDE_SCALING_RATIO=75`.

Before <https://github.com/jp7677/dxvk-nvapi/commit/0bcef80fac77ee7a4f13556228bdd5617b83af53> , `CUSTOM` scales can be set with the key `0x10E41DF5`, thus setting a custom scale of 75% can be achieved with `DXVK_NVAPI_DRS_NGX_DLSS_SR_MODE=custom,DXVK_NVAPI_DRS_SETTINGS=0x10E41DF5=75`.

### Override DLSS-SR scaling ratio

_Available since <https://github.com/jp7677/dxvk-nvapi/commit/0bcef80fac77ee7a4f13556228bdd5617b83af53>_.

This requires [Override DLSS-SR performance mode](https://github.com/jp7677/dxvk-nvapi/wiki/Passing-driver-settings#override-dlss-sr-performance-mode) to be set to `CUSTOM`.

- Key: `NGX_DLSS_SR_OVERRIDE_SCALING_RATIO`
- Values:
  - _`<VALUE>`_
  - `MIN`
  - `MAX`
  - `DEFAULT`

Example 1: `DXVK_NVAPI_DRS_SETTINGS=ngx_dlss_sr_override_scaling_ratio=75`

Example 2: `DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE_SCALING_RATIO=75`

Valid _VALUEs_ are from 33 to 100.

### Enable DLSS-SR DLL/Snippet override

- Key: `NGX_DLSS_SR_OVERRIDE`
- Values:
  - `OFF`
  - `ON`
  - `DEFAULT`

Example 1: `DXVK_NVAPI_DRS_SETTINGS=ngx_dlss_sr_override=on`

Example 2: `DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE=on`

### Override DLSS-SR presets

_Presets K-O available since <https://github.com/jp7677/dxvk-nvapi/commit/84238d8b9d4f245c1cdc987dabb3d6ff18f78705>_.

- Key: `NGX_DLSS_SR_OVERRIDE_RENDER_PRESET_SELECTION`
- Values:
  - `OFF`
  - `RENDER_PRESET_A`
  - `RENDER_PRESET_B`
  - `RENDER_PRESET_C`
  - `RENDER_PRESET_D`
  - `RENDER_PRESET_E`
  - `RENDER_PRESET_F`
  - `RENDER_PRESET_G`
  - `RENDER_PRESET_H`
  - `RENDER_PRESET_I`
  - `RENDER_PRESET_J`
  - `RENDER_PRESET_K`
  - `RENDER_PRESET_L`
  - `RENDER_PRESET_M`
  - `RENDER_PRESET_N`
  - `RENDER_PRESET_O`
  - `RENDER_PRESET_LATEST`
  - `DEFAULT`

Example 1: `DXVK_NVAPI_DRS_SETTINGS=ngx_dlss_sr_override_render_preset_selection=render_preset_latest`

Example 2: `DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE_RENDER_PRESET_SELECTION=render_preset_latest`

### Override DLSS-RR performance mode

- Key: `NGX_DLSS_RR_MODE`
- Values:
  - `PERFORMANCE`
  - `BALANCED`
  - `QUALITY`
  - `SNIPPET_CONTROLLED`
  - `DLAA`
  - `ULTRA_PERFORMANCE`
  - `CUSTOM`
  - `DEFAULT`

Example 1: `DXVK_NVAPI_DRS_SETTINGS=ngx_dlss_rr_mode=dlaa`

Example 2: `DXVK_NVAPI_DRS_NGX_DLSS_RR_MODE=dlaa`

Use [Override DLSS-RR scaling ratio](https://github.com/jp7677/dxvk-nvapi/wiki/Passing-driver-settings#override-dlss-rr-scaling-ratio) for specifying `CUSTOM` scales. Setting a custom scale of 90% can be achieved with `DXVK_NVAPI_DRS_NGX_DLSS_RR_MODE=custom,NGX_DLSS_RR_OVERRIDE_SCALING_RATIO=90`.

Before <https://github.com/jp7677/dxvk-nvapi/commit/0bcef80fac77ee7a4f13556228bdd5617b83af53> , `CUSTOM` scales can be set with the key `0x10C7D4A2`, thus setting a custom scale of 90% can be achieved with `DXVK_NVAPI_DRS_NGX_DLSS_RR_MODE=custom,DXVK_NVAPI_DRS_SETTINGS=0x10C7D4A2=90`.

### Override DLSS-RR scaling ratio

_Available since <https://github.com/jp7677/dxvk-nvapi/commit/0bcef80fac77ee7a4f13556228bdd5617b83af53>_.

This requires [Override DLSS-RR performance mode](https://github.com/jp7677/dxvk-nvapi/wiki/Passing-driver-settings#override-dlss-rr-performance-mode) to be set to `CUSTOM`.

- Key: `NGX_DLSS_RR_OVERRIDE_SCALING_RATIO`
- Values:
  - _`<VALUE>`_
  - `MIN`
  - `MAX`
  - `DEFAULT`

Example 1: `DXVK_NVAPI_DRS_SETTINGS=ngx_dlss_rr_override_scaling_ratio=75`

Example 2: `DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE_SCALING_RATIO=75`

Valid _VALUEs_ are from 33 to 100.

### Enable DLSS-RR DLL/Snippet override

- Key: `NGX_DLSS_RR_OVERRIDE`
- Values:
  - `OFF`
  - `ON`
  - `DEFAULT`

Example 1: `DXVK_NVAPI_DRS_SETTINGS=ngx_dlss_rr_override=on`

Example 2: `DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE=on`

### Override DLSS-RR preset

_Presets G-O available since <https://github.com/jp7677/dxvk-nvapi/commit/84238d8b9d4f245c1cdc987dabb3d6ff18f78705>_.

- Key: `NGX_DLSS_RR_OVERRIDE_RENDER_PRESET_SELECTION`
- Values:
  - `OFF`
  - `RENDER_PRESET_A`
  - `RENDER_PRESET_B`
  - `RENDER_PRESET_C`
  - `RENDER_PRESET_D`
  - `RENDER_PRESET_E`
  - `RENDER_PRESET_F`
  - `RENDER_PRESET_G`
  - `RENDER_PRESET_H`
  - `RENDER_PRESET_I`
  - `RENDER_PRESET_J`
  - `RENDER_PRESET_K`
  - `RENDER_PRESET_L`
  - `RENDER_PRESET_M`
  - `RENDER_PRESET_N`
  - `RENDER_PRESET_O`
  - `RENDER_PRESET_LATEST`
  - `DEFAULT`

Example 1: `DXVK_NVAPI_DRS_SETTINGS=ngx_dlss_rr_override_render_preset_selection=render_preset_latest`

Example 2: `DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE_RENDER_PRESET_SELECTION=render_preset_latest`

### Override DLSSG multi-frame count

- Key: `NGX_DLSSG_MULTI_FRAME_COUNT`
- Values:
  - _`<VALUE>`_
  - `MIN`
  - `MAX`
  - `DEFAULT`

Example 1: `DXVK_NVAPI_DRS_SETTINGS=ngx_dlssg_multi_frame_count=3`

Example 2: `DXVK_NVAPI_DRS_NGX_DLSSG_MULTI_FRAME_COUNT=3`

Valid _VALUEs_ are: `0` (application-controlled), `1` (2x), `2` (3x) and `3` (4x).

### Enable DLSS-FG DLL/Snippet override

- Key: `NGX_DLSS_FG_OVERRIDE`
- Values:
  - `OFF`
  - `ON`
  - `DEFAULT`

Example 1: `DXVK_NVAPI_DRS_SETTINGS=ngx_dlss_fg_override=on`

Example 2: `DXVK_NVAPI_DRS_NGX_DLSS_FG_OVERRIDE=on`

## NGX snippet updates and preset overrides

Applying driver settings does not override NGX snippets/dlls. Use `PROTON_ENABLE_NGX_UPDATER=1` (See [ngx.html](https://download.nvidia.com/XFree86/Linux-x86_64/570.86.16/README/ngx.html) from the NVIDIA driver documentation) for automatic snippets updates and then add the DRS settings that enable DLSS overrides:

- NGX_DLSS_SR_OVERRIDE=on
- NGX_DLSS_RR_OVERRIDE=on
- NGX_DLSS_FG_OVERRIDE=on

Overriding DLSS snippets for all three mentioned NGX features and force latest preset for DLSS-SR and DLSS-RR looks like this with a one liner suitable for e.g. launch options:

- `PROTON_ENABLE_NGX_UPDATER=1 DXVK_NVAPI_DRS_SETTINGS=NGX_DLSS_SR_OVERRIDE=on,NGX_DLSS_RR_OVERRIDE=on,NGX_DLSS_FG_OVERRIDE=on,NGX_DLSS_SR_OVERRIDE_RENDER_PRESET_SELECTION=render_preset_latest,NGX_DLSS_RR_OVERRIDE_RENDER_PRESET_SELECTION=render_preset_latest`

or like this with separate environment variables:

- `PROTON_ENABLE_NGX_UPDATER=1`
- `DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE=on`
- `DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE=on`
- `DXVK_NVAPI_DRS_NGX_DLSS_FG_OVERRIDE=on`
- `DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE_RENDER_PRESET_SELECTION=render_preset_latest`
- `DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE_RENDER_PRESET_SELECTION=render_preset_latest`

or like this when putting those options into Proton's [user_settings.py](https://github.com/ValveSoftware/Proton/blob/experimental_9.0/user_settings.sample.py):

```json
{
    "PROTON_ENABLE_NGX_UPDATER": "1",
    "DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE": "on",
    "DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE": "on",
    "DXVK_NVAPI_DRS_NGX_DLSS_FG_OVERRIDE": "on",
    "DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE_RENDER_PRESET_SELECTION": "render_preset_latest",
    "DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE_RENDER_PRESET_SELECTION": "render_preset_latest"
}
```

Note that `PROTON_ENABLE_NGX_UPDATER` had some issues before R570 driver release and there's still some chance that it won't work in all titles. If you start seeing games quietly failing to launch, this variable may be the cause.

DXVK-NVAPI [logging](https://github.com/jp7677/dxvk-nvapi/blob/master/README.md#tweaks-debugging-and-troubleshooting) shows which settings have been provided from the environment and, more important, which settings have been requested from an application.

As a side note, NGX offers registry settings for enabling graphical indicators for [DLSS](https://github.com/NVIDIA/DLSS/tree/main/utils) and [DLSSG](https://github.com/NVIDIAGameWorks/Streamline/blob/main/docs/ProgrammingGuideDLSS_G.md#200-dlss-fg-indicator-text). Those can be used to validate a different snippet or preset. For convenience, those registry settings can be set with a [DXVK-NVAPI tweak](https://github.com/jp7677/dxvk-nvapi?tab=readme-ov-file#tweaks-debugging-and-troubleshooting): `DXVK_NVAPI_SET_NGX_DEBUG_OPTIONS=DLSSIndicator=1024,DLSSGIndicator=2` for showing indicators and `DXVK_NVAPI_SET_NGX_DEBUG_OPTIONS=DLSSIndicator=0,DLSSGIndicator=0` for hiding the indicators. Be aware, this tweak permanently modifies the registry. The NGX indicators are only active when DLSS is active, which usually means that entering in-game is needed. Once activated, it looks like this:

![image](https://github.com/user-attachments/assets/415a74fb-c051-431d-90ff-6768b751fde2)

(Bottom left and top left)
