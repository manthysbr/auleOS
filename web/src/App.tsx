import { Route, Switch } from 'wouter'
import Login from '@/pages/Login'
import Workspace from '@/pages/Workspace'

function App() {
  return (
    <Switch>
      <Route path="/" component={Login} />
      <Route path="/workspace" component={Workspace} />
      <Route>404: No such agent</Route>
    </Switch>
  )
}

export default App
