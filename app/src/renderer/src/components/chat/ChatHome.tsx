export default function ChatHome() {
  return (
    <div className="flex flex-col items-center py-10 w-full h-full">
      <style>
        {`
          :root {
            --direction: -1;
          }
        `}
      </style>
      <div className="flex flex-col flex-1 max-w-2xl  w-full">
        <div className="flex-1 overflow-y-auto p-4">
          <h1 className="text-2xl font-bold text-black">Chat Home</h1>
        </div>
      </div>
    </div>
  )
}
