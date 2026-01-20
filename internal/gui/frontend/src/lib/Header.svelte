<script>
  import { onMount, onDestroy, createEventDispatcher } from 'svelte'
  import { GetGPUInfo, GetCPUInfo, GetLogo } from '../../wailsjs/go/gui/App'

  const dispatch = createEventDispatcher()

  const logoText = 'Spela'

  let logoSource = ''
  let graphicsInfo = null
  let processorInfo = null
  let refreshTimer = null

  onMount(() => {
    loadLogo()
    refreshMetrics()
    refreshTimer = setInterval(refreshMetrics, 2000)
  })

  onDestroy(() => {
    if (refreshTimer) {
      clearInterval(refreshTimer)
    }
  })

  async function loadLogo() {
    logoSource = await GetLogo()
  }

  async function refreshMetrics() {
    graphicsInfo = await GetGPUInfo()
    processorInfo = await GetCPUInfo()
  }

  function openOptions() {
    dispatch('options')
  }
</script>

<header class="app-header">
  <div class="logo" aria-label="Spela logo">
    {#if logoSource}
      <img src={logoSource} alt="Spela logo" />
    {:else}
      <span class="logo-text">{logoText}</span>
    {/if}
  </div>
  <div class="header-right">
    <div class="metrics">
      <div class="metric-line">
        <span class="metric-label">GPU:</span>
        <span class="metric-value">
          {#if graphicsInfo}
            {graphicsInfo.temperature}C {graphicsInfo.utilization}% {graphicsInfo.powerDraw.toFixed(0)}W
          {:else}
            N/A
          {/if}
        </span>
      </div>
      <div class="metric-line">
        <span class="metric-label">VRAM:</span>
        <span class="metric-value">
          {#if graphicsInfo}
            {(graphicsInfo.memoryUsed / 1024).toFixed(1)}/{(graphicsInfo.memoryTotal / 1024).toFixed(1)} GB
          {:else}
            N/A
          {/if}
        </span>
      </div>
      <div class="metric-line">
        <span class="metric-label">CPU:</span>
        <span class="metric-value">
          {#if processorInfo}
            {processorInfo.utilizationPercent.toFixed(0)}% {processorInfo.averageFrequency}MHz
          {:else}
            N/A
          {/if}
        </span>
      </div>
      <div class="metric-line">
        <span class="metric-label">RAM:</span>
        <span class="metric-value">
          {#if processorInfo}
            {(processorInfo.memoryUsedMegabytes / 1024).toFixed(1)}/{(processorInfo.memoryTotalMegabytes / 1024).toFixed(1)} GB
          {:else}
            N/A
          {/if}
        </span>
      </div>
    </div>
    <button type="button" class="options-button" on:click={openOptions}>Options</button>
  </div>
</header>

<style>
  .app-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1.5rem;
    padding: 0.75rem 1.5rem 1rem;
    border-bottom: 1px solid var(--border-default);
    background-color: var(--bg-primary);
  }

  .logo {
    display: flex;
    align-items: center;
    min-height: 48px;
  }

  .logo img {
    height: 48px;
    width: auto;
    display: block;
  }

  .logo-text {
    font-size: 1.4rem;
    font-weight: 700;
    color: var(--accent-primary);
    letter-spacing: 0.08em;
    text-transform: uppercase;
    font-family: var(--font-ui, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif);
  }

  .header-right {
    display: flex;
    align-items: flex-start;
    gap: 1rem;
  }

  .metrics {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    font-size: 0.75rem;
    align-items: flex-end;
    letter-spacing: 0.02em;
  }

  .metric-line {
    display: flex;
    gap: 0.5rem;
    font-family: var(--font-ui, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif);
  }

  .metric-label {
    color: var(--text-dim);
    text-transform: uppercase;
  }

  .metric-value {
    color: var(--text-primary);
    font-weight: 600;
    white-space: nowrap;
  }

  .options-button {
    padding: 0.45rem 0.9rem;
    border: 1px solid var(--border-default);
    border-radius: 6px;
    background-color: transparent;
    color: var(--text-primary);
    font-size: 0.75rem;
    cursor: pointer;
    align-self: flex-start;
    font-family: var(--font-ui, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif);
  }

  .options-button:hover {
    border-color: var(--border-focus);
    color: var(--accent-primary);
  }

  @media (max-width: 1100px) {
    .header-right {
      flex-direction: column;
      align-items: flex-end;
    }
  }

  @media (max-width: 720px) {
    .app-header {
      flex-direction: column;
      gap: 1rem;
    }

    .header-right {
      width: 100%;
      align-items: flex-start;
    }

    .metrics {
      align-items: flex-start;
    }
  }
</style>
