const { notarize } = require('@electron/notarize')

exports.default = async function notarizeApp(context) {
  if (context.electronPlatformName !== 'darwin') return
  const { appOutDir, packager } = context
  const appName = packager.appInfo.productFilename
  const appBundleId = packager.appInfo.appId // robust fallback

  console.log('🪄 Running notarization...')
  console.log(`📦 App: ${appName}`)
  console.log(`🆔 Bundle ID: ${appBundleId}`)

  return notarize({
    tool: 'notarytool',
    appBundleId,
    appPath: `${appOutDir}/${appName}.app`,
    ascProvider: process.env.NOTARY_TEAM_ID,
    appleApiKey: './build/AuthKey.p8',
    appleApiKeyId: process.env.NOTARY_API_KEY_ID,
    appleApiIssuer: process.env.NOTARY_API_ISSUER
  })
}
