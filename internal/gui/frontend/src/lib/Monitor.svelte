<script>
  import { onMount, onDestroy } from 'svelte'
  import { GetGPUInfo, GetCPUInfo } from '../../wailsjs/go/main/App'

  let gpu = null
  let cpuInfo = null
  let interval

  onMount(async () => {
    await refresh()
    interval = setInterval(refresh, 1000)
  })

  onDestroy(() => {
    if (interval) clearInterval(interval)
  })

  async function refresh() {
    gpu = await GetGPUInfo()
    cpuInfo = await GetCPUInfo()
  }
</script>

<div class="monitor">
  <div class="card">
    <h2>GPU</h2>
    {#if gpu}
      <div class="name">{gpu.name}</div>
      <div class="metrics">
        <div class="metric">
          <span class="label">Temperature</span>
          <span class="value">{gpu.temperature}°C</span>
        </div>
        <div class="metric">
          <span class="label">Utilization</span>
          <span class="value">{gpu.utilization}%</span>
        </div>
        <div class="metric">
          <span class="label">Power</span>
          <span class="value">{gpu.powerDraw.toFixed(0)} / {gpu.powerLimit.toFixed(0)} W</span>
        </div>
        <div class="metric">
          <span class="label">Memory</span>
          <span class="value">{gpu.memoryUsed} / {gpu.memoryTotal} MB</span>
        </div>
        <div class="metric">
          <span class="label">Graphics clk</span>
          <span class="value">{gpu.graphicsClock} MHz</span>
        </div>
        <div class="metric">
          <span class="label">Memory clk</span>
          <span class="value">{gpu.memoryClock} MHz</span>
        </div>
      </div>
    {:else}
      <div class="unavailable">GPU metrics unavailable</div>
    {/if}
  </div>

  <div class="card">
    <h2>CPU</h2>
    {#if cpuInfo}
      <div class="name">{cpuInfo.model}</div>
      <div class="metrics">
        <div class="metric">
          <span class="label">Cores</span>
          <span class="value">{cpuInfo.cores}</span>
        </div>
        <div class="metric">
          <span class="label">Avg frequency</span>
          <span class="value">{cpuInfo.averageFrequency} MHz</span>
        </div>
        <div class="metric">
          <span class="label">Governor</span>
          <span class="value">{cpuInfo.governor}</span>
        </div>
        <div class="metric">
          <span class="label">SMT</span>
          <span class="value">{cpuInfo.smtEnabled ? 'enabled' : 'disabled'}</span>
        </div>
      </div>
    {:else}
      <div class="unavailable">CPU metrics unavailable</div>
    {/if}
  </div>
</div>

<style>
  .monitor {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
    gap: 1rem;
  }

  .card {
    background-color: #232f3e;
    border-radius: 8px;
    padding: 1.5rem;
  }

  h2 {
    color: #76b900;
    margin-bottom: 0.5rem;
  }

  .name {
    color: #8899a6;
    font-size: 0.9rem;
    margin-bottom: 1rem;
  }

  .metrics {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 1rem;
  }

  .metric {
    display: flex;
    flex-direction: column;
  }

  .metric .label {
    color: #8899a6;
    font-size: 0.8rem;
    margin-bottom: 0.25rem;
  }

  .metric .value {
    color: #e0e0e0;
    font-size: 1.1rem;
    font-weight: 500;
  }

  .unavailable {
    color: #8899a6;
    font-style: italic;
  }
</style>
