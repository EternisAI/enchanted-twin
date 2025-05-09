import { useEffect } from 'react'
import QRCode from 'react-qr-code'
import { Loader2, PhoneOff, Smartphone } from 'lucide-react'
import { useMutation, useQuery } from '@apollo/client'

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card'
import { Button } from '../ui/button'
import WhatsAppIcon from '../../assets/icons/whatsapp'
import {
  GetWhatsAppStatusDocument,
  StartWhatsAppConnectionDocument
} from '@renderer/graphql/generated/graphql'

export default function WhatsApp() {
  const { data, loading, error, refetch } = useQuery(GetWhatsAppStatusDocument, {
    pollInterval: 15000
  })
  const [startConnection, { loading: startingConnection }] = useMutation(
    StartWhatsAppConnectionDocument,
    {
      onCompleted: () => {
        refetch()
      }
    }
  )

  const qrCodeData = data?.getWhatsAppStatus?.qrCodeData
  const isConnected = data?.getWhatsAppStatus?.isConnected
  const statusMessage = data?.getWhatsAppStatus?.statusMessage || ''

  useEffect(() => {
    refetch()
  }, [refetch])

  const handleConnect = async () => {
    try {
      await startConnection()
    } catch (error) {
      console.error('Failed to start WhatsApp connection:', error)
    }
  }

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <WhatsAppIcon className="h-5 w-5 text-green-500" />
            WhatsApp
          </CardTitle>
          <CardDescription>Connect your WhatsApp account</CardDescription>
        </CardHeader>
        <CardContent className="flex justify-center items-center py-8">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </CardContent>
      </Card>
    )
  }

  if (error) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <WhatsAppIcon className="h-5 w-5 text-green-500" />
            WhatsApp
          </CardTitle>
          <CardDescription>Connect your WhatsApp account</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="text-destructive">Error loading WhatsApp status</div>
          <Button onClick={() => refetch()} variant="outline" className="mt-4">
            Retry
          </Button>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <WhatsAppIcon className="h-5 w-5 text-green-500" />
          WhatsApp
        </CardTitle>
        <CardDescription>Connect your WhatsApp account</CardDescription>
      </CardHeader>
      <CardContent>
        {isConnected ? (
          <div className="flex flex-col items-center justify-center py-6">
            <div className="flex items-center gap-2 text-green-500 mb-4">
              <Smartphone className="h-8 w-8" />
              <span className="text-lg font-medium">Connected</span>
            </div>
            <p className="text-muted-foreground text-center">{statusMessage}</p>
          </div>
        ) : (
          <div className="flex flex-col items-center">
            {qrCodeData ? (
              <>
                <div className="mb-4 p-4 bg-white rounded-lg">
                  <QRCode value={qrCodeData} size={200} />
                </div>
                <p className="text-sm text-center text-muted-foreground mb-4">
                  Scan this QR code with your WhatsApp app to connect
                </p>
              </>
            ) : (
              <div className="flex flex-col items-center justify-center py-6">
                <PhoneOff className="h-8 w-8 text-muted-foreground mb-4" />
                <p className="text-muted-foreground text-center mb-4">{statusMessage}</p>
                <Button onClick={handleConnect} disabled={startingConnection}>
                  {startingConnection ? (
                    <>
                      <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                      Connecting...
                    </>
                  ) : (
                    'Connect WhatsApp'
                  )}
                </Button>
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
