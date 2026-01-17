<script>
  import { createEventDispatcher, onMount } from 'svelte'
  import { GetProfile, SaveProfile, CheckDLLUpdates, UpdateDLLs, RestoreDLLs, HasDLLBackup } from '../../wailsjs/go/main/App'

  export let game

  const dispatch = createEventDispatcher()

  let profile = null
  let saving = false
  let message = ''
  let dllUpdates = []
  let hasBackup = false
  let updatingDLLs = false
  let restoringDLLs = false

  const presetOptions = ['performance', 'balanced', 'quality', 'custom']
  const srModeOptions = ['off', 'ultra_performance', 'performance', 'balanced', 'quality', 'dlaa']

  onMount(async () => {
    await Promise.all([loadProfile(), checkDLLUpdates()])
  })

  async function loadProfile() {
    profile = await GetProfile(game.appId)
    if (!profile) {
      profile = {
        preset: 'balanced',
        srMode: 'balanced',
        srOverride: false,
        fgEnabled: false,
        enableHdr: false,
        enableWayland: false,
        enableNgxUpdater: false
      }
    }
  }

  async function checkDLLUpdates() {
    dllUpdates = await CheckDLLUpdates(game.appId) || []
    hasBackup = await HasDLLBackup(game.appId)
  }

  function formatError(e) {
    if (typeof e === 'string') return e
    if (e?.message) return e.message
    return String(e)
  }

  async function save() {
    saving = true
    try {
      await SaveProfile(game.appId, profile)
      message = 'Profile saved!'
      setTimeout(() => message = '', 3000)
    } catch (e) {
      message = 'Failed to save: ' + formatError(e)
    }
    saving = false
  }

  async function updateDLLs() {
    updatingDLLs = true
    try {
      await UpdateDLLs(game.appId)
      message = 'DLLs updated!'
      await checkDLLUpdates()
      setTimeout(() => message = '', 3000)
    } catch (e) {
      message = 'Failed to update: ' + formatError(e)
    }
    updatingDLLs = false
  }

  async function restoreDLLs() {
    restoringDLLs = true
    try {
      await RestoreDLLs(game.appId)
      message = 'DLLs restored!'
      await checkDLLUpdates()
      setTimeout(() => message = '', 3000)
    } catch (e) {
      message = 'Failed to restore: ' + formatError(e)
    }
    restoringDLLs = false
  }

  $: hasUpdates = dllUpdates.some(d => d.hasUpdate)
</script>

<div class="detail">
  <button class="back" on:click={() => dispatch('back')}>
    ← Back
  </button>

  <h1>{game.name}</h1>

  <div class="info">
    <div class="row">
      <span class="label">App ID</span>
      <span class="value">{game.appId}</span>
    </div>
    <div class="row">
      <span class="label">Install dir</span>
      <span class="value">{game.installDir}</span>
    </div>
    {#if game.prefixPath}
      <div class="row">
        <span class="label">Prefix</span>
        <span class="value">{game.prefixPath}</span>
      </div>
    {/if}
  </div>

  {#if game.dlls && game.dlls.length > 0}
    <div class="section">
      <h2>Detected DLLs</h2>
      <div class="dll-list">
        {#each dllUpdates as dll}
          <div class="dll">
            <span class="dll-name">{dll.name}</span>
            <span class="dll-version">
              {dll.currentVersion || 'unknown'}
              {#if dll.hasUpdate}
                <span class="update-badge">→ {dll.latestVersion}</span>
              {/if}
            </span>
          </div>
        {/each}
      </div>
      <div class="dll-actions">
        {#if hasUpdates}
          <button class="update-btn" on:click={updateDLLs} disabled={updatingDLLs}>
            {updatingDLLs ? 'Updating...' : 'Update all DLLs'}
          </button>
        {/if}
        {#if hasBackup}
          <button class="restore-btn" on:click={restoreDLLs} disabled={restoringDLLs}>
            {restoringDLLs ? 'Restoring...' : 'Restore original DLLs'}
          </button>
        {/if}
      </div>
    </div>
  {/if}

  {#if profile}
    <div class="section">
      <h2>Profile settings</h2>

      <div class="form">
        <div class="field">
          <label for="preset">Preset</label>
          <select id="preset" bind:value={profile.preset}>
            {#each presetOptions as opt}
              <option value={opt}>{opt}</option>
            {/each}
          </select>
        </div>

        <div class="field">
          <label for="srMode">DLSS-SR mode</label>
          <select id="srMode" bind:value={profile.srMode}>
            {#each srModeOptions as opt}
              <option value={opt}>{opt}</option>
            {/each}
          </select>
        </div>

        <div class="field checkbox">
          <input type="checkbox" id="srOverride" bind:checked={profile.srOverride} />
          <label for="srOverride">DLSS-SR override</label>
        </div>

        <div class="field checkbox">
          <input type="checkbox" id="fgEnabled" bind:checked={profile.fgEnabled} />
          <label for="fgEnabled">Frame generation</label>
        </div>

        <div class="field checkbox">
          <input type="checkbox" id="enableHdr" bind:checked={profile.enableHdr} />
          <label for="enableHdr">HDR</label>
        </div>

        <div class="field checkbox">
          <input type="checkbox" id="enableWayland" bind:checked={profile.enableWayland} />
          <label for="enableWayland">Wayland</label>
        </div>

        <div class="field checkbox">
          <input type="checkbox" id="enableNgxUpdater" bind:checked={profile.enableNgxUpdater} />
          <label for="enableNgxUpdater">NGX Updater</label>
        </div>

        <button class="save" on:click={save} disabled={saving}>
          {saving ? 'Saving...' : 'Save profile'}
        </button>

        {#if message}
          <div class="message">{message}</div>
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  .detail {
    max-width: 800px;
  }

  .back {
    padding: 0.5rem 1rem;
    border: none;
    border-radius: 4px;
    background-color: transparent;
    color: #3d8bff;
    cursor: pointer;
    font-size: 0.9rem;
    margin-bottom: 1rem;
  }

  .back:hover {
    background-color: #232f3e;
  }

  h1 {
    font-size: 1.5rem;
    margin-bottom: 1rem;
  }

  h2 {
    font-size: 1.1rem;
    color: #76b900;
    margin-bottom: 0.75rem;
  }

  .info {
    background-color: #232f3e;
    border-radius: 4px;
    padding: 1rem;
    margin-bottom: 1.5rem;
  }

  .row {
    display: flex;
    margin-bottom: 0.5rem;
  }

  .row:last-child {
    margin-bottom: 0;
  }

  .label {
    width: 100px;
    color: #8899a6;
  }

  .value {
    color: #e0e0e0;
    word-break: break-all;
  }

  .section {
    margin-bottom: 1.5rem;
  }

  .dll-list {
    background-color: #232f3e;
    border-radius: 4px;
    padding: 0.5rem;
  }

  .dll {
    display: flex;
    justify-content: space-between;
    padding: 0.5rem;
  }

  .dll-name {
    color: #e0e0e0;
  }

  .dll-version {
    color: #76b900;
  }

  .update-badge {
    color: #ffcc00;
    margin-left: 0.5rem;
  }

  .dll-actions {
    display: flex;
    gap: 0.5rem;
    margin-top: 0.75rem;
  }

  .update-btn, .restore-btn {
    padding: 0.5rem 1rem;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.9rem;
  }

  .update-btn {
    background-color: #76b900;
    color: black;
  }

  .update-btn:hover:not(:disabled) {
    background-color: #8dcf00;
  }

  .restore-btn {
    background-color: #3d4f5f;
    color: #e0e0e0;
  }

  .restore-btn:hover:not(:disabled) {
    background-color: #4d5f6f;
  }

  .update-btn:disabled, .restore-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .form {
    background-color: #232f3e;
    border-radius: 4px;
    padding: 1rem;
  }

  .field {
    margin-bottom: 1rem;
  }

  .field label {
    display: block;
    color: #8899a6;
    margin-bottom: 0.25rem;
    font-size: 0.9rem;
  }

  .field select {
    width: 100%;
    padding: 0.5rem;
    border: 1px solid #3d4f5f;
    border-radius: 4px;
    background-color: #1b2636;
    color: #e0e0e0;
  }

  .field.checkbox {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .field.checkbox label {
    margin-bottom: 0;
  }

  .field.checkbox input {
    width: 18px;
    height: 18px;
  }

  .save {
    width: 100%;
    padding: 0.75rem;
    border: none;
    border-radius: 4px;
    background-color: #3d8bff;
    color: white;
    cursor: pointer;
    font-size: 1rem;
    margin-top: 0.5rem;
  }

  .save:hover:not(:disabled) {
    background-color: #2d7bef;
  }

  .save:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .message {
    margin-top: 0.75rem;
    padding: 0.5rem;
    background-color: #76b900;
    color: black;
    border-radius: 4px;
    text-align: center;
  }
</style>
