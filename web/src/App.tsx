import { Route, Routes } from 'react-router'
import LaunchDetail from './captain/LaunchDetail'
import Launches from './captain/Launches'
import NewLaunch from './captain/NewLaunch'
import Repos from './captain/Repos'
import Approve from './crew/Approve'
import Done from './crew/Done'
import Fallback from './crew/Fallback'
import Invite from './crew/Invite'
import OpenLink from './crew/OpenLink'
import Show from './crew/Show'
import Settings from './settings/Settings'
import Home from './Home'

export default function App() {
  return (
    <Routes>
      <Route path="/l/:openToken" element={<OpenLink />} />
      <Route path="/l/:openToken/show" element={<Show />} />
      <Route path="/s/:token" element={<Invite />} />
      <Route path="/s/:token/approve" element={<Approve />} />
      <Route path="/s/:token/done" element={<Done />} />
      <Route path="/s/:token/fallback" element={<Fallback />} />
      <Route path="/settings" element={<Settings />} />
      <Route path="/opt-out" element={<Settings />} />
      <Route path="/captain" element={<Launches />} />
      <Route path="/captain/new" element={<NewLaunch />} />
      <Route path="/captain/launches/:id" element={<LaunchDetail />} />
      <Route path="/captain/repos" element={<Repos />} />
      <Route path="*" element={<Home />} />
    </Routes>
  )
}
