console.log("[wisebot1] Starting...")
let cont = 0
const wait = Math.random() * 2000

setInterval(() => {
  console.log(`[wisebot1] ${cont++}`)
}, wait)
