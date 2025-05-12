const { notarize } = require('@electron/notarize')

exports.default = async function notarizeApp(context) {
  const { appOutDir, electronPlatformName } = context
  if (electronPlatformName !== 'darwin') return

  const appName = context.packager.appInfo.productFilename
  console.log('🪄 Running custom notarize script…')

  return notarize({
    tool: 'notarytool',
    provider: 'api',
    appBundleId: 'com.eternis.enchanted',
    appPath: `${appOutDir}/${appName}.app`,
    ascProvider: process.env.NOTARY_TEAM_ID,
    appleApiKey: './build/AuthKey.p8',
    appleApiKeyId: process.env.NOTARY_API_KEY_ID,
    appleApiIssuer: process.env.NOTARY_API_ISSUER
  })
}
