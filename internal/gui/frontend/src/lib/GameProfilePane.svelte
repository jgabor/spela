<script>
  import { createEventDispatcher } from 'svelte'
  import Dropdown from './Dropdown.svelte'
  import NumberField from './NumberField.svelte'
  import ProfileFieldMeta from './ProfileFieldMeta.svelte'
  import {
    srModeOptions,
    srPresetOptions,
    rrPresetOptions,
    multiFrameOptions,
    powerMizerOptions,
    vrrOptions,
    frameGenerationOptions,
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

  // Labels and descriptions come from the Go field descriptors through the
  // projection, so this pane keeps no wording of its own for them.
  $: labelFor = (property) => semanticsByField[profileFieldByProperty[property]]?.label ?? ''
  $: descriptionFor = (property) => semanticsByField[profileFieldByProperty[property]]?.description ?? ''
  $: valueTextFor = (property) => semanticsByField[profileFieldByProperty[property]]?.valueText ?? ''
  $: unitFor = (property) => semanticsByField[profileFieldByProperty[property]]?.unit ?? ''
  $: hintFor = (property) => semanticsByField[profileFieldByProperty[property]]?.hint ?? ''

  function forwardPatchAction(event) {
    dispatch('patchAction', event.detail)
  }

  function forwardApplyOwnValue(event) {
    dispatch('applyOwnValue', event.detail)
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
    // The count is the largest of the fields; the confirm names the games of all of them.
    const mostGames = items.reduce((most, item) => (item.ownValueCount ?? 0) > (most.ownValueCount ?? 0) ? item : most, items[0])
    return {
      ...items[0],
      source: items.some(item => item.source === 'override') ? 'override' : items[0].source,
      ownValueSuffix: mostGames.ownValueSuffix,
      ownValueCount: mostGames.ownValueCount ?? 0,
      ownValueAction: mostGames.ownValueAction
    }
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
  <p class="default-note">This game has no profile of its own; every value follows All games.</p>
{/if}
<div class="profile-grid" on:change={handleNativeFieldChange}>
  {#if showSubsystem(1)}
  <div class="section boxed">
    <h2>DLSS settings</h2>

    <div class="form">
      <div class="field">
        <label for="srMode">{labelFor('srMode')}</label>
        <Dropdown
          storedLabel={valueTextFor('srMode')}
          bind:value={profile.srMode}
          options={srModeOptions}
          on:change={() => fieldChanged('dlss.sr_mode')}
        />
        <span class="hint">{descriptionFor('srMode')}</span>
        <ProfileFieldMeta field="dlss.sr_mode" semantic={semanticsByField['dlss.sr_mode']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field">
        <label for="srPreset">{labelFor('srPreset')}</label>
        <Dropdown
          storedLabel={valueTextFor('srPreset')}
          bind:value={profile.srPreset}
          options={srPresetOptions}
          on:change={() => fieldChanged('dlss.sr_preset')}
        />
        <span class="hint">{descriptionFor('srPreset')}</span>
        <ProfileFieldMeta field="dlss.sr_preset" semantic={semanticsByField['dlss.sr_preset']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="srOverride" bind:checked={profile.srOverride} />
        <label for="srOverride">{labelFor('srOverride')}</label>
        <span class="hint">{descriptionFor('srOverride')}</span>
        <ProfileFieldMeta field="dlss.sr_override" semantic={semanticsByField['dlss.sr_override']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="indicator" bind:checked={profile.indicator} />
        <label for="indicator">{labelFor('indicator')}</label>
        <span class="hint">{descriptionFor('indicator')}</span>
        <ProfileFieldMeta field="dlss.indicator" semantic={semanticsByField['dlss.indicator']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field">
        <label for="rrMode">{labelFor('rrMode')}</label>
        <Dropdown storedLabel={valueTextFor('rrMode')} bind:value={profile.rrMode} options={srModeOptions} on:change={() => fieldChanged('dlss.rr_mode')} />
        <span class="hint">{descriptionFor('rrMode')}</span>
        <ProfileFieldMeta field="dlss.rr_mode" semantic={semanticsByField['dlss.rr_mode']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field">
        <label for="rrPreset">{labelFor('rrPreset')}</label>
        <Dropdown storedLabel={valueTextFor('rrPreset')} bind:value={profile.rrPreset} options={rrPresetOptions} on:change={() => fieldChanged('dlss.rr_preset')} />
        <span class="hint">{descriptionFor('rrPreset')}</span>
        <ProfileFieldMeta field="dlss.rr_preset" semantic={semanticsByField['dlss.rr_preset']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="rrOverride" bind:checked={profile.rrOverride} />
        <label for="rrOverride">{labelFor('rrOverride')}</label>
        <span class="hint">{descriptionFor('rrOverride')}</span>
        <ProfileFieldMeta field="dlss.rr_override" semantic={semanticsByField['dlss.rr_override']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field">
        <label for="fgEnabled">{labelFor('fgEnabled')}</label>
        <Dropdown
          bind:value={frameGenerationMode}
          options={frameGenerationOptions}
          on:change={(event) => updateFrameGeneration(event.detail)}
        />
        <span class="hint">{descriptionFor('fgEnabled')}</span>
        <ProfileFieldMeta fields={frameGenerationFields} semantic={frameGenerationSemantic} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="fgIndicator" bind:checked={profile.fgIndicator} />
        <label for="fgIndicator">{labelFor('fgIndicator')}</label>
        <span class="hint">{descriptionFor('fgIndicator')}</span>
        <ProfileFieldMeta field="dlss.fg_indicator" semantic={semanticsByField['dlss.fg_indicator']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field">
        <label for="multiFrame">{labelFor('multiFrame')}</label>
        <Dropdown
          storedLabel={valueTextFor('multiFrame')}
          bind:value={profile.multiFrame}
          options={multiFrameOptions}
          on:change={() => fieldChanged('dlss.multi_frame')}
        />
        <span class="hint">{descriptionFor('multiFrame')}</span>
        <ProfileFieldMeta field="dlss.multi_frame" semantic={semanticsByField['dlss.multi_frame']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
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
        <label for="shaderCache">{labelFor('shaderCache')}</label>
        <span class="hint">{descriptionFor('shaderCache')}</span>
        <ProfileFieldMeta field="gpu.shader_cache" semantic={semanticsByField['gpu.shader_cache']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field">
        <label for="shaderCachePath">{labelFor('shaderCachePath')}</label>
        <input type="text" id="shaderCachePath" bind:value={profile.shaderCachePath} placeholder="Empty path" />
        <span class="hint">{descriptionFor('shaderCachePath')}</span>
        <ProfileFieldMeta field="gpu.shader_cache_path" semantic={semanticsByField['gpu.shader_cache_path']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="threadedOptimization" bind:checked={profile.threadedOptimization} />
        <label for="threadedOptimization">{labelFor('threadedOptimization')}</label>
        <span class="hint">{descriptionFor('threadedOptimization')}</span>
        <ProfileFieldMeta field="gpu.threaded_optimization" semantic={semanticsByField['gpu.threaded_optimization']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field">
        <label for="powerMizer">{labelFor('powerMizer')}</label>
        <Dropdown
          storedLabel={valueTextFor('powerMizer')}
          bind:value={profile.powerMizer}
          options={powerMizerOptions}
          on:change={() => fieldChanged('gpu.power_mizer')}
        />
        <span class="hint">{descriptionFor('powerMizer')}</span>
        <ProfileFieldMeta field="gpu.power_mizer" semantic={semanticsByField['gpu.power_mizer']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field">
        <label for="vrr">{labelFor('vrr')}</label>
        <Dropdown
          storedLabel={valueTextFor('vrr')}
          bind:value={profile.vrr}
          options={vrrOptions}
          placeholder="Unconfigured — leave KDE unchanged"
          on:change={() => fieldChanged('gpu.vrr')}
        />
        <span class="hint">{descriptionFor('vrr')}</span>
        <ProfileFieldMeta field="gpu.vrr" semantic={semanticsByField['gpu.vrr']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field">
        <label for="clockOffset">{labelFor('clockOffset')}</label>
        <NumberField
          id="clockOffset"
          bind:value={profile.clockOffset}
          unit={unitFor('clockOffset')}
          hint={hintFor('clockOffset')}
          on:commit={() => fieldChanged('gpu.clock_offset')}
        />
        <span class="hint">{descriptionFor('clockOffset')}</span>
        <ProfileFieldMeta field="gpu.clock_offset" semantic={semanticsByField['gpu.clock_offset']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field">
        <label for="memoryOffset">{labelFor('memoryOffset')}</label>
        <NumberField
          id="memoryOffset"
          bind:value={profile.memoryOffset}
          unit={unitFor('memoryOffset')}
          hint={hintFor('memoryOffset')}
          on:commit={() => fieldChanged('gpu.memory_offset')}
        />
        <span class="hint">{descriptionFor('memoryOffset')}</span>
        <ProfileFieldMeta field="gpu.memory_offset" semantic={semanticsByField['gpu.memory_offset']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field">
        <label for="powerLimit">{labelFor('powerLimit')}</label>
        <NumberField
          id="powerLimit"
          bind:value={profile.powerLimit}
          unit={unitFor('powerLimit')}
          hint={hintFor('powerLimit')}
          on:commit={() => fieldChanged('gpu.power_limit')}
        />
        <span class="hint">{descriptionFor('powerLimit')}</span>
        <ProfileFieldMeta field="gpu.power_limit" semantic={semanticsByField['gpu.power_limit']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field">
        <label for="fanSpeed">{labelFor('fanSpeed')}</label>
        <NumberField
          id="fanSpeed"
          bind:value={profile.fanSpeed}
          unit={unitFor('fanSpeed')}
          hint={hintFor('fanSpeed')}
          on:commit={() => fieldChanged('gpu.fan_speed')}
        />
        <span class="hint">{descriptionFor('fanSpeed')}</span>
        <ProfileFieldMeta field="gpu.fan_speed" semantic={semanticsByField['gpu.fan_speed']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>
    </div>
  </div>

  {/if}
  {#if showSubsystem(3)}
  <div class="section boxed">
    <h2>CPU settings</h2>

    <div class="form">
      <div class="field">
        <label for="governor">{labelFor('governor')}</label>
        <Dropdown
          storedLabel={valueTextFor('governor')}
          bind:value={profile.governor}
          options={governorOptions}
          on:change={() => fieldChanged('cpu.governor')}
        />
        <span class="hint">{descriptionFor('governor')}</span>
        <ProfileFieldMeta field="cpu.governor" semantic={semanticsByField['cpu.governor']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field">
        <label for="smt">{labelFor('smt')}</label>
        <Dropdown
          storedLabel={valueTextFor('smt')}
          bind:value={profile.smt}
          options={smtOptions}
          on:change={() => fieldChanged('cpu.smt')}
        />
        <span class="hint">{descriptionFor('smt')}</span>
        <ProfileFieldMeta field="cpu.smt" semantic={semanticsByField['cpu.smt']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field">
        <label for="affinity">{labelFor('affinity')}</label>
        <input type="text" id="affinity" bind:value={profile.affinity} placeholder="No affinity" />
        <span class="hint">{descriptionFor('affinity')}</span>
        <ProfileFieldMeta field="cpu.affinity" semantic={semanticsByField['cpu.affinity']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
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
        <label for="enableHdr">{labelFor('enableHdr')}</label>
        <span class="hint">{descriptionFor('enableHdr')}</span>
        <ProfileFieldMeta field="proton.enable_hdr" semantic={semanticsByField['proton.enable_hdr']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="enableWayland" bind:checked={profile.enableWayland} />
        <label for="enableWayland">{labelFor('enableWayland')}</label>
        <span class="hint">{descriptionFor('enableWayland')}</span>
        <ProfileFieldMeta field="proton.enable_wayland" semantic={semanticsByField['proton.enable_wayland']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="enableNgxUpdater" bind:checked={profile.enableNgxUpdater} />
        <label for="enableNgxUpdater">{labelFor('enableNgxUpdater')}</label>
        <span class="hint">{descriptionFor('enableNgxUpdater')}</span>
        <ProfileFieldMeta field="proton.enable_ngx_updater" semantic={semanticsByField['proton.enable_ngx_updater']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>

      <div class="field checkbox">
        <input type="checkbox" id="vkd3dHeap" bind:checked={profile.vkd3dHeap} />
        <label for="vkd3dHeap">{labelFor('vkd3dHeap')}</label>
        <span class="hint">{descriptionFor('vkd3dHeap')}</span>
        <ProfileFieldMeta field="proton.vkd3d_heap" semantic={semanticsByField['proton.vkd3d_heap']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
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
        <label for="overlayEnabled">{labelFor('overlayEnabled')}</label>
        <span class="hint">{descriptionFor('overlayEnabled')}</span>
        <ProfileFieldMeta field="overlay.enabled" semantic={semanticsByField['overlay.enabled']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>
      <div class="field">
        <label for="overlayPosition">{labelFor('overlayPosition')}</label>
        <Dropdown storedLabel={valueTextFor('overlayPosition')} bind:value={profile.overlayPosition} options={overlayPositionOptions} on:change={() => fieldChanged('overlay.position')} />
        <span class="hint">{descriptionFor('overlayPosition')}</span>
        <ProfileFieldMeta field="overlay.position" semantic={semanticsByField['overlay.position']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>
      <div class="field checkbox">
        <input type="checkbox" id="overlayShowFps" bind:checked={profile.overlayShowFps} />
        <label for="overlayShowFps">{labelFor('overlayShowFps')}</label>
        <span class="hint">{descriptionFor('overlayShowFps')}</span>
        <ProfileFieldMeta field="overlay.show_fps" semantic={semanticsByField['overlay.show_fps']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>
      <div class="field checkbox">
        <input type="checkbox" id="overlayShowFrametime" bind:checked={profile.overlayShowFrametime} />
        <label for="overlayShowFrametime">{labelFor('overlayShowFrametime')}</label>
        <span class="hint">{descriptionFor('overlayShowFrametime')}</span>
        <ProfileFieldMeta field="overlay.show_frametime" semantic={semanticsByField['overlay.show_frametime']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>
      <div class="field checkbox">
        <input type="checkbox" id="overlayShowCpu" bind:checked={profile.overlayShowCpu} />
        <label for="overlayShowCpu">{labelFor('overlayShowCpu')}</label>
        <span class="hint">{descriptionFor('overlayShowCpu')}</span>
        <ProfileFieldMeta field="overlay.show_cpu" semantic={semanticsByField['overlay.show_cpu']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>
      <div class="field checkbox">
        <input type="checkbox" id="overlayShowGpu" bind:checked={profile.overlayShowGpu} />
        <label for="overlayShowGpu">{labelFor('overlayShowGpu')}</label>
        <span class="hint">{descriptionFor('overlayShowGpu')}</span>
        <ProfileFieldMeta field="overlay.show_gpu" semantic={semanticsByField['overlay.show_gpu']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>
      <div class="field checkbox">
        <input type="checkbox" id="overlayShowVram" bind:checked={profile.overlayShowVram} />
        <label for="overlayShowVram">{labelFor('overlayShowVram')}</label>
        <span class="hint">{descriptionFor('overlayShowVram')}</span>
        <ProfileFieldMeta field="overlay.show_vram" semantic={semanticsByField['overlay.show_vram']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
      </div>
      <div class="field">
        <label for="overlayToggleKey">{labelFor('overlayToggleKey')}</label>
        <input type="text" id="overlayToggleKey" bind:value={profile.overlayToggleKey} placeholder="No key" />
        <span class="hint">{descriptionFor('overlayToggleKey')}</span>
        <ProfileFieldMeta field="overlay.toggle_key" semantic={semanticsByField['overlay.toggle_key']} {profileMode} on:action={forwardPatchAction} on:applyOwnValue={forwardApplyOwnValue} />
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

  :global(.field.checkbox .meta-block) {
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
