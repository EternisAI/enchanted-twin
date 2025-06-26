package drive

// func TestDriveProcessor_SyncWithStoredToken(t *testing.T) {
// 	ctx := context.Background()

// 	store, err := db.NewStore(ctx, "../../../output/sqlite/store.db")
// 	require.NoError(t, err)
// 	defer store.Close()

// 	logger := log.NewWithOptions(os.Stdout, log.Options{
// 		ReportCaller:    true,
// 		ReportTimestamp: true,
// 		Level:           log.DebugLevel,
// 		TimeFormat:      time.Kitchen,
// 		Prefix:          "[drive-test] ",
// 	})

// 	tokens, err := store.GetOAuthTokens(
// 		ctx,
// 		"google",
// 	)

// 	driveProcessor, err := NewDriveProcessor(store, logger)
// 	logger.Info("Tokens", "tokens", tokens)
// 	require.NoError(t, err)

// 	records, _, err := driveProcessor.Sync(ctx, tokens.AccessToken)
// 	require.NoError(t, err)

// 	logger.Info("Records", "records", records)
// }
