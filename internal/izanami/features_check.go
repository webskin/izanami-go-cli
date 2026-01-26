package izanami

// This file is intentionally empty.
// Feature check operations have been moved to feature_check_client.go
// as part of the AdminClient/FeatureCheckClient separation.
//
// For feature check operations, use:
//   - izanami.NewFeatureCheckClient(cfg) to create a client
//   - izanami.CheckFeature(client, ctx, ...) for single feature checks
//   - izanami.CheckFeatures(client, ctx, ...) for bulk feature checks
