<script>
  import { createEventDispatcher } from 'svelte'
  import Dropdown from './Dropdown.svelte'
  import ProfileFieldMeta from './ProfileFieldMeta.svelte'
  import {
    srModeOptions,
    srPresetOptions,
    rrPresetOptions,
    multiFrameOptions,
    powerMizerOptions,
    frameGenerationOptions,
    clockOffsetOptions,
    memoryOffsetOptions,
    governorOptions,
    smtOptions,
    overlayPositionOptions,
    profileFieldByProperty
  } from './profileFieldOptions.js'

  export let profile
  export let profileMode = 'game'
  export let profileSubsystem = null
  export let game = null
  export let desktop
  export let pendingOperations = {}

  const dispatch = createEventDispatcher()

  $: showSubsystem = (index) => profileSubsystem === null || profileSubsystem === index

  let frameGenerationMode = 'no_override'
  const frameGenerationFields = ['dlss.fg_enabled', 'dlss.fg_override']
  let semanticsByField = {}
  let vkd3dHeapNotice = ''

  $: if (profile) {
    frameGenerationMode = profile.fgOverride
      ? (profile.fgEnabled ? 'enabled' : 'disabled')
      : 'no_override'
  }

  $: semanticsByField = Object.fromEntries((profile?.semantics || []).map(item => [item.field, item]))
  $: frameGenerationSemantic = compositeSemantic(frameGenerationFields, semanticsByField)
  $: frameGenerationPending = compositePending(frameGenerationFields, pendingOperations)

  function forwardPatchAction(event) {
    dispatch('patchAction', event.detail)
  }

  function fieldChanged(...fields) {
    dispatch('fieldChange', { fields })
  }

  function handleNativeFieldChange(event) {
    const field = profileFieldByProperty[event.target.id]
    if (field) fieldChanged(field)
  }

  function compositeSemantic(fields, semantics) {
    const items = fields.map(field => semantics[field]).filter(Boolean)
    if (!items.length) return null
    return { ...items[0], source: items.some(item => item.source === 'override') ? 'override' : items[0].source }
  }

  function compositePending(fields, operations) {
    const operation = operations[fields[0]]
    return operation && fields.every(field => operations[field] === operation) ? operation : ''
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
    profile.fgOverride = value !== 'no_override'
    profile.fgEnabled = value === 'enabled'
    fieldChanged('dlss.fg_enabled', 'dlss.fg_override')
  }
</script>

{#if profileMode === 'game' && profile.inheritedFromDefault}
  <p class="default-note">Using default profile values.</p>
{/if}
<div class="profile-grid" on:change={handleNativeFieldChange}>
  {#if showSubsystem(1)}
  <div class="section boxed">
    <h2>DLSS settings</h2>

    <div class="form">
      <div class="field">
        <label for="srMode">Quality mode</label>
        <Dropdown
          bind:value={profile.srMode}
          options={srModeOptions}
          on:change={() => fieldChanged('dlss.sr_mode')}
        />
        <span class="hint">Resolution preset for DLSS super resolution.</span>
        <ProfileFieldMeta field="dlss.sr_mode" semantic={semanticsByField['dlss.sr_mode']} {profileMode} pendingOperation={pendingOperations['dlss.sr_mode']} on:action={forwardPatchAction} />
      </div>

      <div class="field">
        <label for="srPreset">DLSS preset</label>
        <Dropdown
          bind:value={profile.srPreset}
          options={srPresetOptions}
          on:change={() => fieldChanged('dlss.sr_preset')}
        />
        <span class="hint">auto: mode-linked transformer; A-F: CNN (DLSS 2/3); J-M: Transformer (DLSS 4/4.5)</span>
        <ProfileFieldMeta field="dlss.sr_preset" semantic={semanticsByField['dlss.sr_preset']} {profileMode} pendingOperation={pendingOperations['dlss.sr_preset']} on:action={forwardPatchAction} />
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="srOverride" bind:checked={profile.srOverride} />
        <label for="srOverride">Override (force DLSS even if unsupported)</label>
        <span class="hint">Use DLSS even if the game does not expose it.</span>
        <ProfileFieldMeta field="dlss.sr_override" semantic={semanticsByField['dlss.sr_override']} {profileMode} pendingOperation={pendingOperations['dlss.sr_override']} on:action={forwardPatchAction} />
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="indicator" bind:checked={profile.indicator} />
        <label for="indicator">Show DLSS indicator</label>
        <span class="hint">Display a small on-screen DLSS status overlay.</span>
        <ProfileFieldMeta field="dlss.indicator" semantic={semanticsByField['dlss.indicator']} {profileMode} pendingOperation={pendingOperations['dlss.indicator']} on:action={forwardPatchAction} />
      </div>

      <div class="field">
        <label for="rrMode">Ray reconstruction mode</label>
        <Dropdown bind:value={profile.rrMode} options={srModeOptions} on:change={() => fieldChanged('dlss.rr_mode')} />
        <span class="hint">DLSS ray reconstruction quality preset.</span>
        <ProfileFieldMeta field="dlss.rr_mode" semantic={semanticsByField['dlss.rr_mode']} {profileMode} pendingOperation={pendingOperations['dlss.rr_mode']} on:action={forwardPatchAction} />
      </div>

      <div class="field">
        <label for="rrPreset">Ray reconstruction preset</label>
        <Dropdown bind:value={profile.rrPreset} options={rrPresetOptions} on:change={() => fieldChanged('dlss.rr_preset')} />
        <ProfileFieldMeta field="dlss.rr_preset" semantic={semanticsByField['dlss.rr_preset']} {profileMode} pendingOperation={pendingOperations['dlss.rr_preset']} on:action={forwardPatchAction} />
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="rrOverride" bind:checked={profile.rrOverride} />
        <label for="rrOverride">RR override</label>
        <ProfileFieldMeta field="dlss.rr_override" semantic={semanticsByField['dlss.rr_override']} {profileMode} pendingOperation={pendingOperations['dlss.rr_override']} on:action={forwardPatchAction} />
      </div>

      <div class="field">
        <label for="fgEnabled">Frame generation</label>
        <Dropdown
          bind:value={frameGenerationMode}
          options={frameGenerationOptions}
          on:change={(event) => updateFrameGeneration(event.detail)}
        />
        <span class="hint">Generate extra frames for higher FPS.</span>
        <ProfileFieldMeta fields={frameGenerationFields} semantic={frameGenerationSemantic} {profileMode} pendingOperation={frameGenerationPending} on:action={forwardPatchAction} />
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="fgIndicator" bind:checked={profile.fgIndicator} />
        <label for="fgIndicator">Show frame generation indicator</label>
        <ProfileFieldMeta field="dlss.fg_indicator" semantic={semanticsByField['dlss.fg_indicator']} {profileMode} pendingOperation={pendingOperations['dlss.fg_indicator']} on:action={forwardPatchAction} />
      </div>

      <div class="field">
        <label for="multiFrame">Multi-frame generation</label>
        <Dropdown
          bind:value={profile.multiFrame}
          options={multiFrameOptions}
          on:change={() => fieldChanged('dlss.multi_frame')}
        />
        <span class="hint">Extra frames to generate (0=off).</span>
        <ProfileFieldMeta field="dlss.multi_frame" semantic={semanticsByField['dlss.multi_frame']} {profileMode} pendingOperation={pendingOperations['dlss.multi_frame']} on:action={forwardPatchAction} />
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
        <ProfileFieldMeta field="gpu.shader_cache" semantic={semanticsByField['gpu.shader_cache']} {profileMode} pendingOperation={pendingOperations['gpu.shader_cache']} on:action={forwardPatchAction} />
      </div>

      <div class="field">
        <label for="shaderCachePath">Shader cache path</label>
        <input type="text" id="shaderCachePath" bind:value={profile.shaderCachePath} placeholder="Empty path" />
        <ProfileFieldMeta field="gpu.shader_cache_path" semantic={semanticsByField['gpu.shader_cache_path']} {profileMode} pendingOperation={pendingOperations['gpu.shader_cache_path']} on:action={forwardPatchAction} />
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="threadedOptimization" bind:checked={profile.threadedOptimization} />
        <label for="threadedOptimization">Threaded optimization</label>
        <span class="hint">Use multi-core rendering when supported.</span>
        <ProfileFieldMeta field="gpu.threaded_optimization" semantic={semanticsByField['gpu.threaded_optimization']} {profileMode} pendingOperation={pendingOperations['gpu.threaded_optimization']} on:action={forwardPatchAction} />
      </div>

      <div class="field">
        <label for="powerMizer">Power mode</label>
        <Dropdown
          bind:value={profile.powerMizer}
          options={powerMizerOptions}
          on:change={() => fieldChanged('gpu.power_mizer')}
        />
        <span class="hint">GPU power policy for the game.</span>
        <ProfileFieldMeta field="gpu.power_mizer" semantic={semanticsByField['gpu.power_mizer']} {profileMode} pendingOperation={pendingOperations['gpu.power_mizer']} on:action={forwardPatchAction} />
      </div>

      <div class="field">
        <label for="clockOffset">Clock offset</label>
        <Dropdown
          bind:value={profile.clockOffset}
          options={clockOffsetOptions}
          on:change={() => fieldChanged('gpu.clock_offset')}
        />
        <span class="hint">GPU core clock offset in MHz.</span>
        <ProfileFieldMeta field="gpu.clock_offset" semantic={semanticsByField['gpu.clock_offset']} {profileMode} pendingOperation={pendingOperations['gpu.clock_offset']} on:action={forwardPatchAction} />
      </div>

      <div class="field">
        <label for="memoryOffset">Memory offset</label>
        <Dropdown
          bind:value={profile.memoryOffset}
          options={memoryOffsetOptions}
          on:change={() => fieldChanged('gpu.memory_offset')}
        />
        <span class="hint">GPU memory clock offset in MHz.</span>
        <ProfileFieldMeta field="gpu.memory_offset" semantic={semanticsByField['gpu.memory_offset']} {profileMode} pendingOperation={pendingOperations['gpu.memory_offset']} on:action={forwardPatchAction} />
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
          on:change={() => fieldChanged('cpu.governor')}
        />
        <span class="hint">CPU frequency scaling governor for the game.</span>
        <ProfileFieldMeta field="cpu.governor" semantic={semanticsByField['cpu.governor']} {profileMode} pendingOperation={pendingOperations['cpu.governor']} on:action={forwardPatchAction} />
      </div>

      <div class="field">
        <label for="smt">SMT</label>
        <Dropdown
          bind:value={profile.smt}
          options={smtOptions}
          on:change={() => fieldChanged('cpu.smt')}
        />
        <span class="hint">Simultaneous multi-threading (hyperthreading).</span>
        <ProfileFieldMeta field="cpu.smt" semantic={semanticsByField['cpu.smt']} {profileMode} pendingOperation={pendingOperations['cpu.smt']} on:action={forwardPatchAction} />
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
        <ProfileFieldMeta field="proton.enable_hdr" semantic={semanticsByField['proton.enable_hdr']} {profileMode} pendingOperation={pendingOperations['proton.enable_hdr']} on:action={forwardPatchAction} />
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="enableWayland" bind:checked={profile.enableWayland} />
        <label for="enableWayland">Wayland</label>
        <span class="hint">Prefer native Wayland when available.</span>
        <ProfileFieldMeta field="proton.enable_wayland" semantic={semanticsByField['proton.enable_wayland']} {profileMode} pendingOperation={pendingOperations['proton.enable_wayland']} on:action={forwardPatchAction} />
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="enableNgxUpdater" bind:checked={profile.enableNgxUpdater} />
        <label for="enableNgxUpdater">NGX Updater</label>
        <span class="hint">Allow Proton to update DLSS DLLs.</span>
        <ProfileFieldMeta field="proton.enable_ngx_updater" semantic={semanticsByField['proton.enable_ngx_updater']} {profileMode} pendingOperation={pendingOperations['proton.enable_ngx_updater']} on:action={forwardPatchAction} />
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="vkd3dHeap" bind:checked={profile.vkd3dHeap} />
        <label for="vkd3dHeap">VKD3D Heap</label>
        <span class="hint">Enable the VKD3D descriptor heap code path (VKD3D_CONFIG=descriptor_heap). Requires Proton-CachyOS 10.0-20260321+ or 11.0+ and NVIDIA driver 580.94.16+.</span>
        <ProfileFieldMeta field="proton.vkd3d_heap" semantic={semanticsByField['proton.vkd3d_heap']} {profileMode} pendingOperation={pendingOperations['proton.vkd3d_heap']} on:action={forwardPatchAction} />
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
        <ProfileFieldMeta field="overlay.enabled" semantic={semanticsByField['overlay.enabled']} {profileMode} pendingOperation={pendingOperations['overlay.enabled']} on:action={forwardPatchAction} />
      </div>
      <div class="field">
        <label for="overlayPosition">Position</label>
        <Dropdown bind:value={profile.overlayPosition} options={overlayPositionOptions} on:change={() => fieldChanged('overlay.position')} />
        <ProfileFieldMeta field="overlay.position" semantic={semanticsByField['overlay.position']} {profileMode} pendingOperation={pendingOperations['overlay.position']} on:action={forwardPatchAction} />
      </div>
      <div class="field checkbox">
        <input type="checkbox" id="overlayShowFps" bind:checked={profile.overlayShowFps} />
        <label for="overlayShowFps">Show FPS</label>
        <ProfileFieldMeta field="overlay.show_fps" semantic={semanticsByField['overlay.show_fps']} {profileMode} pendingOperation={pendingOperations['overlay.show_fps']} on:action={forwardPatchAction} />
      </div>
      <div class="field checkbox">
        <input type="checkbox" id="overlayShowFrametime" bind:checked={profile.overlayShowFrametime} />
        <label for="overlayShowFrametime">Show frametime</label>
        <ProfileFieldMeta field="overlay.show_frametime" semantic={semanticsByField['overlay.show_frametime']} {profileMode} pendingOperation={pendingOperations['overlay.show_frametime']} on:action={forwardPatchAction} />
      </div>
      <div class="field checkbox">
        <input type="checkbox" id="overlayShowCpu" bind:checked={profile.overlayShowCpu} />
        <label for="overlayShowCpu">Show CPU</label>
        <ProfileFieldMeta field="overlay.show_cpu" semantic={semanticsByField['overlay.show_cpu']} {profileMode} pendingOperation={pendingOperations['overlay.show_cpu']} on:action={forwardPatchAction} />
      </div>
      <div class="field checkbox">
        <input type="checkbox" id="overlayShowGpu" bind:checked={profile.overlayShowGpu} />
        <label for="overlayShowGpu">Show GPU</label>
        <ProfileFieldMeta field="overlay.show_gpu" semantic={semanticsByField['overlay.show_gpu']} {profileMode} pendingOperation={pendingOperations['overlay.show_gpu']} on:action={forwardPatchAction} />
      </div>
      <div class="field checkbox">
        <input type="checkbox" id="overlayShowVram" bind:checked={profile.overlayShowVram} />
        <label for="overlayShowVram">Show VRAM</label>
        <ProfileFieldMeta field="overlay.show_vram" semantic={semanticsByField['overlay.show_vram']} {profileMode} pendingOperation={pendingOperations['overlay.show_vram']} on:action={forwardPatchAction} />
      </div>
      <div class="field">
        <label for="overlayToggleKey">Toggle key</label>
        <input type="text" id="overlayToggleKey" bind:value={profile.overlayToggleKey} placeholder="No key" />
        <ProfileFieldMeta field="overlay.toggle_key" semantic={semanticsByField['overlay.toggle_key']} {profileMode} pendingOperation={pendingOperations['overlay.toggle_key']} on:action={forwardPatchAction} />
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

  :global(.field.checkbox .profile-meta) {
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
