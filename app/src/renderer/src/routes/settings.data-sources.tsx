import { createFileRoute } from '@tanstack/react-router'
import { DataSourcesPanel } from '@renderer/components/data-sources/DataSourcesPanel'
import MCPPanel from '@renderer/components/oauth/MCPPanel'
import ConnectedMCPPanel from '@renderer/components/oauth/ConnectedMCPPanel'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@renderer/components/ui/tabs'
import ConnectMCPServerButton from '@renderer/components/oauth/MCPConnectServerButton'
import { FolderSyncIcon, NetworkIcon, PlugIcon } from 'lucide-react'
import LocalFolderSync from '@renderer/components/data-sources/LocalFolderSync'
import { SettingsContent } from '@renderer/components/settings/SettingsContent'
import { motion, AnimatePresence } from 'framer-motion'
import { useState } from 'react'

export const Route = createFileRoute('/settings/data-sources')({
  component: ImportDataSettings
})

function ImportDataSettings() {
  const [activeTab, setActiveTab] = useState('available')

  const tabVariants = {
    hidden: { opacity: 0 },
    visible: { opacity: 1 },
    exit: { opacity: 0 }
  }

  return (
    <SettingsContent>
      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <div className="flex flex-col sm:flex-row gap-2 w-full items-center justify-between  pb-10">
          <TabsList>
            <TabsTrigger value="available">
              <NetworkIcon className="w-4 h-4" /> Available
            </TabsTrigger>
            <TabsTrigger value="local-files">
              <FolderSyncIcon className="w-4 h-4" /> Synced Folders
            </TabsTrigger>
            <TabsTrigger value="connected">
              <PlugIcon className="w-4 h-4" /> Connected
            </TabsTrigger>
          </TabsList>
          <ConnectMCPServerButton onSuccess={() => {}} />
        </div>
        <AnimatePresence mode="wait">
          <TabsContent value="available" key="available">
            {activeTab === 'available' && (
              <motion.div
                className="flex flex-col gap-15"
                variants={tabVariants}
                initial="hidden"
                animate="visible"
                exit="exit"
                transition={{ duration: 0.2, ease: 'easeInOut' }}
              >
                <MCPPanel />
                <DataSourcesPanel />
              </motion.div>
            )}
          </TabsContent>
          <TabsContent value="local-files" key="local-files">
            {activeTab === 'local-files' && (
              <motion.div
                variants={tabVariants}
                initial="hidden"
                animate="visible"
                exit="exit"
                transition={{ duration: 0.2, ease: 'easeInOut' }}
              >
                <LocalFolderSync />
              </motion.div>
            )}
          </TabsContent>
          <TabsContent value="connected" key="connected">
            {activeTab === 'connected' && (
              <motion.div
                variants={tabVariants}
                initial="hidden"
                animate="visible"
                exit="exit"
                transition={{ duration: 0.2, ease: 'easeInOut' }}
              >
                <ConnectedMCPPanel />
              </motion.div>
            )}
          </TabsContent>
        </AnimatePresence>
      </Tabs>
    </SettingsContent>
  )
}
