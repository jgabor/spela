// The names the interface prints for the DLL families.
const labels = {
  dlss: 'DLSS',
  dlssg: 'DLSS-G',
  dlssd: 'DLSS-D',
  xess: 'XeSS',
  fsr: 'FSR'
}

export function dllLabel(type) {
  return labels[type] || String(type).toUpperCase()
}
