<script>
  import { onMount } from 'svelte'

  export let desktop
  export let section = 0

  let games = []
  let message = ''

  onMount(load)

  async function load() {
    try {
      games = await desktop.GetGames()
    } catch (e) {
      message = String(e)
    }
  }
</script>

<div class="dll-catalog">
  <h1>{section === 0 ? 'DLL Library' : 'Deployment matrix'}</h1>
  {#if message}
    <p class="error">{message}</p>
  {:else if section === 0}
    <p class="hint">Cached DLL inventory — use TUI DLL Catalog for full library details.</p>
  {:else}
    <table>
      <thead>
        <tr><th>Game</th><th>DLLs</th></tr>
      </thead>
      <tbody>
        {#each games as game}
          <tr>
            <td>{game.name}</td>
            <td>{game.dlls?.length ? game.dlls.map(d => d.type).join(', ') : '—'}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</div>

<style>
  .dll-catalog { padding: 1rem; overflow: auto; height: 100%; }
  h1 { font-size: 1.1rem; color: var(--accent-primary); margin-bottom: 0.75rem; }
  table { width: 100%; border-collapse: collapse; font-family: var(--font-mono); font-size: 0.8rem; }
  th, td { border: 1px solid var(--border-default); padding: 0.35rem 0.5rem; text-align: left; }
  .hint, .error { color: var(--text-dim); font-size: 0.85rem; }
</style>
