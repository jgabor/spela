function hasDLLs(game) {
  return Array.isArray(game.dlls) && game.dlls.length > 0
}

function matchesSearch(game, searchQuery) {
  return !searchQuery || game.name.toLowerCase().includes(searchQuery.toLowerCase())
}

export function applyFiltersAndSort(list, searchQuery, dllFilter, profileFilter, sort) {
  const filtered = list.filter(game => {
    if (!matchesSearch(game, searchQuery)) {
      return false
    }
    if (dllFilter && !hasDLLs(game)) {
      return false
    }
    if (profileFilter && !game.hasProfile) {
      return false
    }
    return true
  })
  return sortGames(filtered, sort)
}

export function sortGames(list, sort) {
  const sorted = [...list]
  switch (sort) {
    case 'name-asc':
      sorted.sort((a, b) => a.name.localeCompare(b.name))
      break
    case 'name-desc':
      sorted.sort((a, b) => b.name.localeCompare(a.name))
      break
    case 'dlls-first':
      sorted.sort((a, b) => {
        const aHas = hasDLLs(a)
        const bHas = hasDLLs(b)
        if (aHas !== bHas) return bHas ? 1 : -1
        return a.name.localeCompare(b.name)
      })
      break
    case 'profile-first':
      sorted.sort((a, b) => {
        if (a.hasProfile !== b.hasProfile) return b.hasProfile ? 1 : -1
        return a.name.localeCompare(b.name)
      })
      break
  }
  return sorted
}

export function planBatchDLLUpdate(list, selected) {
  const selectedGames = list.filter(game => selected.has(game.appId))
  const eligible = selectedGames.filter(hasDLLs)
  const skipped = selectedGames.filter(game => !hasDLLs(game))
  return { eligible, skipped }
}

function plural(count, singular, pluralForm = `${singular}s`) {
  return count === 1 ? singular : pluralForm
}

export function formatBatchDLLResult(successCount, failCount, skippedCount) {
  const parts = [`Updated ${successCount} ${plural(successCount, 'game')}`]
  if (failCount > 0) {
    parts.push(`${failCount} failed`)
  }
  if (skippedCount > 0) {
    parts.push(`${skippedCount} skipped`)
  }
  return parts.join(', ')
}
