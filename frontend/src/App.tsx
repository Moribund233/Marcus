import { useState } from 'react'
import { TitleBar } from '@/components/title-bar'
import { Sidebar } from '@/components/sidebar'
import { ToolGrid } from '@/components/tool-grid'

function App() {
  const [category, setCategory] = useState('all')

  return (
    <div className="flex h-screen flex-col bg-background">
      <TitleBar />
      <div className="flex flex-1 overflow-hidden">
        <Sidebar active={category} onSelect={setCategory} />
        <ToolGrid category={category} />
      </div>
    </div>
  )
}

export default App
