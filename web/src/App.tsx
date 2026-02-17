import { Route, Switch } from 'wouter'
import Login from '@/pages/Login'
import DesktopShell from '@/pages/DesktopShell'

function App() {
  return (
    <Switch>
      <Route path="/" component={Login} />
      <Route path="/workspace" component={DesktopShell} />
      <Route>404: No such agent</Route>
    </Switch>
  )
}

export default App
