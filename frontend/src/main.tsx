import { createRoot } from 'react-dom/client'
import './Styles/global.css'
import App from './App'
import { CrashBoundary } from './Components'

const container = document.getElementById('root')
const root = createRoot(container!)

root.render(
  <CrashBoundary>
    <App />
  </CrashBoundary>,
)
