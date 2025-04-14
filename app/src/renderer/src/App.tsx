import Versions from './components/Versions'
import electronLogo from './assets/electron.svg'
import { ApolloClientProvider } from './graphql/provider'
import ChatContainer from './components/chat/ChatContainer'

function App(): React.JSX.Element {
  const ipcHandle = (): void => window.electron.ipcRenderer.send('ping')

  return (
    <>
      <ApolloClientProvider>
        <img alt="logo" className="logo" src={electronLogo} />
        {/* <p className="tip">
        Please try pressing <code>F12</code> to open the devTool
      </p> */}
        <div className="actions">
          <div className="action">
            <a target="_blank" rel="noreferrer" onClick={ipcHandle}>
              Send IPC
            </a>
            <ChatContainer />
          </div>
        </div>
        <Versions></Versions>
      </ApolloClientProvider>
    </>
  )
}

export default App
