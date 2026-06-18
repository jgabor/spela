<script>
  import Dropdown from './Dropdown.svelte'
  import {
    srModeOptions,
    srPresetOptions,
    multiFrameOptions,
    powerMizerOptions,
    frameGenerationOptions,
    clockOffsetOptions,
    memoryOffsetOptions,
    governorOptions,
    smtOptions,
    overlayPositionOptions
  } from './profileFieldOptions.js'

  export let profile
  export let profileMode = 'game'
  export let profileSubsystem = null
  export let game = null
  export let desktop

  $: showSubsystem = (index) => profileSubsystem === null || profileSubsystem === index

  let frameGenerationMode = '(default)'
  let semanticsByField = {}
  let vkd3dHeapNotice = ''

  $: if (profile) {
    frameGenerationMode = profile.fgOverride
      ? (profile.fgEnabled ? 'true' : 'false')
      : '(default)'
  }

  $: semanticsByField = Object.fromEntries((profile?.semantics || []).map(item => [item.field, item]))

  function semanticText(field) {
    const semantic = semanticsByField[field]
    if (!semantic) {
      return ''
    }
    if (profileMode === 'default') {
      return `impact ${semantic.impact} · restore ${semantic.restore}`
    }
    return `source ${semantic.source} · impact ${semantic.impact} · restore ${semantic.restore}`
  }

  async function refreshVkd3dHeapNotice() {
    if (profileMode !== 'game' || !game || !profile?.vkd3dHeap) {
      vkd3dHeapNotice = ''
      return
    }
    try {
      vkd3dHeapNotice = await desktop.VKD3DHeapCompatibilityNotice(game.appId) || ''
    } catch {
      vkd3dHeapNotice = ''
    }
  }

  // Re-check vkd3d_heap compatibility when the toggle flips, the selected
  // game changes, or the editor swaps between default and per-game mode.
  $: {
    const _heap = profile?.vkd3dHeap
    const _appId = game?.appId
    const _mode = profileMode
    void _heap
    void _appId
    void _mode
    void refreshVkd3dHeapNotice()
  }

  function updateFrameGeneration(value) {
    if (!profile) {
      return
    }
    if (value === '(default)') {
      profile.fgOverride = false
      profile.fgEnabled = false
      return
    }
    profile.fgOverride = true
    profile.fgEnabled = value === 'true'
  }
</script>

{#if profileMode === 'game' && profile.inheritedFromDefault}
  <p class="default-note">Using default profile values.</p>
{/if}
<div class="profile-grid">
  {#if showSubsystem(1)}
  <div class="section boxed">
    <h2>DLSS settings</h2>

    <div class="form">
      <div class="field">
        <label for="srMode">Quality mode</label>
        <Dropdown
          bind:value={profile.srMode}
          options={srModeOptions}
        />
        <span class="hint">Resolution preset for DLSS super resolution.</span>
        <span class="profile-meta">{semanticText('dlss.sr_mode')}</span>
      </div>

      <div class="field">
        <label for="srPreset">DLSS preset</label>
        <Dropdown
          bind:value={profile.srPreset}
          options={srPresetOptions}
        />
        <span class="hint">auto: mode-linked transformer; A-F: CNN (DLSS 2/3); J-M: Transformer (DLSS 4/4.5)</span>
        <span class="profile-meta">{semanticText('dlss.sr_preset')}</span>
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="srOverride" bind:checked={profile.srOverride} />
        <label for="srOverride">Override (force DLSS even if unsupported)</label>
        <span class="hint">Use DLSS even if the game does not expose it.</span>
        <span class="profile-meta">{semanticText('dlss.sr_override')}</span>
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="indicator" bind:checked={profile.indicator} />
        <label for="indicator">Show DLSS indicator</label>
        <span class="hint">Display a small on-screen DLSS status overlay.</span>
        <span class="profile-meta">{semanticText('dlss.indicator')}</span>
      </div>

      <div class="field">
        <label for="rrMode">Ray reconstruction mode</label>
        <Dropdown bind:value={profile.rrMode} options={srModeOptions} />
        <span class="hint">DLSS ray reconstruction quality preset.</span>
        <span class="profile-meta">{semanticText('dlss.rr_mode')}</span>
      </div>

      <div class="field">
        <label for="rrPreset">Ray reconstruction preset</label>
        <Dropdown bind:value={profile.rrPreset} options={srPresetOptions} />
        <span class="profile-meta">{semanticText('dlss.rr_preset')}</span>
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="rrOverride" bind:checked={profile.rrOverride} />
        <label for="rrOverride">RR override</label>
        <span class="profile-meta">{semanticText('dlss.rr_override')}</span>
      </div>

      <div class="field">
        <label for="fgEnabled">Frame generation</label>
        <Dropdown
          bind:value={frameGenerationMode}
          options={frameGenerationOptions}
          on:change={(event) => updateFrameGeneration(event.detail)}
        />
        <span class="hint">Generate extra frames for higher FPS.</span>
        <span class="profile-meta">{semanticText('dlss.fg_enabled')}</span>
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="fgIndicator" bind:checked={profile.fgIndicator} />
        <label for="fgIndicator">Show frame generation indicator</label>
        <span class="profile-meta">{semanticText('dlss.fg_indicator')}</span>
      </div>

      <div class="field">
        <label for="multiFrame">Multi-frame generation</label>
        <Dropdown
          bind:value={profile.multiFrame}
          options={multiFrameOptions}
        />
        <span class="hint">Extra frames to generate (0=off).</span>
        <span class="profile-meta">{semanticText('dlss.multi_frame')}</span>
      </div>
    </div>
  </div>

  {/if}
  {#if showSubsystem(2)}
  <div class="section boxed">
    <h2>GPU settings</h2>

    <div class="form">
      <div class="field checkbox">
        <input type="checkbox" id="shaderCache" bind:checked={profile.shaderCache} />
        <label for="shaderCache">Shader cache</label>
        <span class="hint">Enable shader caching for faster reloads.</span>
        <span class="profile-meta">{semanticText('gpu.shader_cache')}</span>
      </div>

      <div class="field">
        <label for="shaderCachePath">Shader cache path</label>
        <input type="text" id="shaderCachePath" bind:value={profile.shaderCachePath} placeholder="(default)" />
        <span class="profile-meta">{semanticText('gpu.shader_cache_path')}</span>
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="threadedOptimization" bind:checked={profile.threadedOptimization} />
        <label for="threadedOptimization">Threaded optimization</label>
        <span class="hint">Use multi-core rendering when supported.</span>
        <span class="profile-meta">{semanticText('gpu.threaded_optimization')}</span>
      </div>

      <div class="field">
        <label for="powerMizer">Power mode</label>
        <Dropdown
          bind:value={profile.powerMizer}
          options={powerMizerOptions}
        />
        <span class="hint">GPU power policy for the game.</span>
        <span class="profile-meta">{semanticText('gpu.power_mizer')}</span>
      </div>

      <div class="field">
        <label for="clockOffset">Clock offset</label>
        <Dropdown
          bind:value={profile.clockOffset}
          options={clockOffsetOptions}
        />
        <span class="hint">GPU core clock offset in MHz.</span>
        <span class="profile-meta">{semanticText('gpu.clock_offset')}</span>
      </div>

      <div class="field">
        <label for="memoryOffset">Memory offset</label>
        <Dropdown
          bind:value={profile.memoryOffset}
          options={memoryOffsetOptions}
        />
        <span class="hint">GPU memory clock offset in MHz.</span>
        <span class="profile-meta">{semanticText('gpu.memory_offset')}</span>
      </div>
    </div>
  </div>

  {/if}
  {#if showSubsystem(3)}
  <div class="section boxed">
    <h2>CPU settings</h2>

    <div class="form">
      <div class="field">
        <label for="governor">Governor</label>
        <Dropdown
          bind:value={profile.governor}
          options={governorOptions}
        />
        <span class="hint">CPU frequency scaling governor for the game.</span>
        <span class="profile-meta">{semanticText('cpu.governor')}</span>
      </div>

      <div class="field">
        <label for="smt">SMT</label>
        <Dropdown
          bind:value={profile.smt}
          options={smtOptions}
        />
        <span class="hint">Simultaneous multi-threading (hyperthreading).</span>
        <span class="profile-meta">{semanticText('cpu.smt')}</span>
      </div>
    </div>
  </div>

  {/if}
  {#if showSubsystem(0)}
  <div class="section boxed">
    <h2>Proton settings</h2>

    <div class="form">
      <div class="field checkbox">
        <input type="checkbox" id="enableHdr" bind:checked={profile.enableHdr} />
        <label for="enableHdr">HDR</label>
        <span class="hint">Enable HDR output for supported displays.</span>
        <span class="profile-meta">{semanticText('proton.enable_hdr')}</span>
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="enableWayland" bind:checked={profile.enableWayland} />
        <label for="enableWayland">Wayland</label>
        <span class="hint">Prefer native Wayland when available.</span>
        <span class="profile-meta">{semanticText('proton.enable_wayland')}</span>
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="enableNgxUpdater" bind:checked={profile.enableNgxUpdater} />
        <label for="enableNgxUpdater">NGX Updater</label>
        <span class="hint">Allow Proton to update DLSS DLLs.</span>
        <span class="profile-meta">{semanticText('proton.enable_ngx_updater')}</span>
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="vkd3dHeap" bind:checked={profile.vkd3dHeap} />
        <label for="vkd3dHeap">VKD3D Heap</label>
        <span class="hint">Enable the VKD3D descriptor heap code path (PROTON_VKD3D_HEAP=1). Requires a recent Proton-CachyOS build and a current NVIDIA driver.</span>
        <span class="profile-meta">{semanticText('proton.vkd3d_heap')}</span>
        {#if profile.vkd3dHeap && vkd3dHeapNotice}
          <div class="vkd3d-notice" data-level={vkd3dHeapNotice.startsWith('⚠') ? 'warn' : 'info'}>
            {vkd3dHeapNotice}
          </div>
        {/if}
      </div>
    </div>
  </div>
  {/if}
  {#if showSubsystem(4)}
  <div class="section boxed">
    <h2>Overlay settings</h2>
    <div class="form">
      <div class="field checkbox">
        <input type="checkbox" id="overlayEnabled" bind:checked={profile.overlayEnabled} />
        <label for="overlayEnabled">Enable overlay</label>
        <span class="profile-meta">{semanticText('overlay.enabled')}</span>
      </div>
      <div class="field">
        <label for="overlayPosition">Position</label>
        <Dropdown bind:value={profile.overlayPosition} options={overlayPositionOptions} />
        <span class="profile-meta">{semanticText('overlay.position')}</span>
      </div>
      <div class="field checkbox">
        <input type="checkbox" id="overlayShowFps" bind:checked={profile.overlayShowFps} />
        <label for="overlayShowFps">Show FPS</label>
        <span class="profile-meta">{semanticText('overlay.show_fps')}</span>
      </div>
      <div class="field checkbox">
        <input type="checkbox" id="overlayShowFrametime" bind:checked={profile.overlayShowFrametime} />
        <label for="overlayShowFrametime">Show frametime</label>
        <span class="profile-meta">{semanticText('overlay.show_frametime')}</span>
      </div>
      <div class="field checkbox">
        <input type="checkbox" id="overlayShowCpu" bind:checked={profile.overlayShowCpu} />
        <label for="overlayShowCpu">Show CPU</label>
        <span class="profile-meta">{semanticText('overlay.show_cpu')}</span>
      </div>
      <div class="field checkbox">
        <input type="checkbox" id="overlayShowGpu" bind:checked={profile.overlayShowGpu} />
        <label for="overlayShowGpu">Show GPU</label>
        <span class="profile-meta">{semanticText('overlay.show_gpu')}</span>
      </div>
      <div class="field checkbox">
        <input type="checkbox" id="overlayShowVram" bind:checked={profile.overlayShowVram} />
        <label for="overlayShowVram">Show VRAM</label>
        <span class="profile-meta">{semanticText('overlay.show_vram')}</span>
      </div>
      <div class="field">
        <label for="overlayToggleKey">Toggle key</label>
        <input type="text" id="overlayToggleKey" bind:value={profile.overlayToggleKey} placeholder="(default)" />
        <span class="profile-meta">{semanticText('overlay.toggle_key')}</span>
      </div>
    </div>
  </div>
  {/if}

</div>

<style>
  .default-note {
    margin: 0;
    font-size: 0.85rem;
    color: var(--text-dim);
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  h2 {
    font-size: 0.85rem;
    color: var(--accent-secondary);
    margin-bottom: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.12em;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .profile-grid {
    display: grid;
    grid-template-columns: 1fr;
    gap: 1rem;
    margin-top: 0.5rem;
  }

  @media (min-width: 80ch) {
    .profile-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  .section {
    margin-bottom: 1.5rem;
  }

  .profile-grid .section {
    margin-bottom: 0;
  }

  .section.boxed h2 {
    margin-bottom: 0.5rem;
  }

  .section.boxed:last-child {
    margin-bottom: 0;
  }

  .section.boxed {
    border: 1px solid var(--border-default);
    border-radius: 0;
    padding: 0.75rem;
    background-color: var(--bg-secondary);
  }

  .form {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .field {
    margin-bottom: 0;
  }

  .field label {
    display: block;
    color: var(--text-dim);
    margin-bottom: 0.25rem;
    font-size: 0.8rem;
    letter-spacing: 0.02em;
    text-transform: uppercase;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .field.checkbox {
    display: grid;
    grid-template-columns: auto 1fr;
    column-gap: 0.5rem;
    row-gap: 0.2rem;
    align-items: start;
  }

  .field.checkbox label {
    margin-bottom: 0;
    text-transform: none;
    letter-spacing: 0.02em;
  }

  .field.checkbox input {
    width: 18px;
    height: 18px;
    accent-color: var(--accent-primary);
    margin-top: 0.15rem;
  }

  .hint {
    display: block;
    font-size: 0.72rem;
    color: var(--text-dim);
    margin-top: 0.2rem;
    line-height: 1.3;
    text-transform: none;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .field.checkbox .hint {
    grid-column: 2;
  }

  .profile-meta {
    display: block;
    margin-top: 0.2rem;
    color: var(--accent-secondary);
    font-size: 0.68rem;
    line-height: 1.3;
    text-transform: none;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .field.checkbox .profile-meta {
    grid-column: 2;
  }

  .vkd3d-notice {
    grid-column: 2;
    margin-top: 0.3rem;
    padding: 0.35rem 0.55rem;
    border: 1px solid var(--border-default);
    background-color: var(--bg-secondary);
    color: var(--text-dim);
    font-size: 0.75rem;
    line-height: 1.3;
    font-family: var(--font-mono, "JetBrains Mono", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  }

  .vkd3d-notice[data-level="warn"] {
    border-color: var(--warning, var(--error));
    color: var(--warning, var(--error));
  }
</style>
