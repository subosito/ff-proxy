package e2e

//func TestMetrics(t *testing.T) {
//	// Note: This tests the proxy can successfully forward on metrics and target metrics to Saas as an integration test
//	// if SaaS is experiencing high load or a slowdown in processing metrics these tests may time out and fail
//	// in this case either rerun or manually verify everything is fine before skipping
//	if !RunMetricsTests() {
//		t.Skip("skipping metrics tests")
//	}
//	targetIdentifier := "metricsTarget"
//
//	// auth against proxy
//	token, claims, err := testhelpers.AuthenticateSDKClient(GetServerAPIKey(), GetStreamURL(), nil)
//	if err != nil {
//		t.Error(err)
//	}
//
//	timestamp := time.Now().UnixMilli()
//
//	// send metrics request to proxy and get 200 response
//	resp, err := sendMetrics(claims.Environment, token, client.PostMetricsJSONBody{
//		MetricsData: &[]client.MetricsData{{
//			Attributes: []client.KeyValue{{
//				Key:   "featureIdentifier",
//				Value: "bool-flag1",
//			}, {
//				Key:   "featureName",
//				Value: "bool-flag1",
//			}, {
//				Key:   "variationIdentifier",
//				Value: "false",
//			}, {
//				Key:   "featureValue",
//				Value: "false",
//			}, {
//				Key:   "SDK_TYPE",
//				Value: "server",
//			}, {
//				Key:   "SDK_LANGUAGE",
//				Value: "go",
//			}, {
//				Key:   "SDK_VERSION",
//				Value: "1.0.0",
//			}, {
//				Key:   "target",
//				Value: targetIdentifier,
//			}},
//			Count:       1,
//			MetricsType: "FFMETRICS",
//			Timestamp:   timestamp,
//		}},
//		TargetData: &[]client.TargetData{{
//			Attributes: []client.KeyValue{{
//				Key:   "key",
//				Value: "value",
//			}},
//			Identifier: targetIdentifier,
//			Name:       targetIdentifier,
//		}},
//	})
//
//	if err != nil {
//		t.Errorf("metrics request failed: err %s", err)
//	}
//
//	assert.Equal(t, 200, resp.StatusCode())
//
//	// send request to SaaS to get features and check metrics have been sent from proxy and successfully registered for up to 3 minutes
//	metricsSuccess := false
//	for i := 1; i <= 18; i++ {
//		time.Sleep(10 * time.Second)
//		metrics, err := getFeatureFlagMetrics("bool-flag1", GetEnvironmentIdentifier())
//		if err != nil {
//			t.Errorf("failed to fetch flags: err %s", err)
//		}
//
//		// check that flag metrics exist
//		if metrics.Status.Status != "active" {
//			log.Warnf("attempt %d failed, expected status 'active', got %s", i, metrics.Status.Status)
//			continue
//		}
//		if int64(metrics.Status.LastAccess) != timestamp {
//			log.Warnf("attempt %d failed, expected LastAccess %d, got %d", i, timestamp, metrics.Status.LastAccess)
//			continue
//		}
//		metricsSuccess = true
//		log.Info("Detected metrics successfully registered on SaaS")
//		break
//	}
//
//	assert.Equal(t, true, metricsSuccess, "failed to detect metrics registered successfully on SaaS")
//
//	// send request to SaaS to see if target was sent from proxy and successfully registered for up to 2 minutes
//	targetSuccess := false
//	for i := 1; i <= 12; i++ {
//		time.Sleep(10 * time.Second)
//		target, err := GetTarget(context.Background(), targetIdentifier, &admin.GetTargetParams{
//			AccountIdentifier:     GetAccountIdentifier(),
//			OrgIdentifier:         GetOrgIdentifier(),
//			ProjectIdentifier:     GetProjectIdentifier(),
//			EnvironmentIdentifier: GetEnvironmentIdentifier(),
//		})
//		if err != nil {
//			t.Errorf("failed to fetch target: err %s", err)
//		}
//		if target.StatusCode() != 200 {
//			log.Warnf("attempt %d failed, expected 200, got %d", i, target.StatusCode())
//			continue
//		}
//		if target.JSON200.Identifier != targetIdentifier {
//			log.Warnf("attempt %d failed, expected identifier %s, got %s", i, targetIdentifier, target.JSON200.Identifier)
//			continue
//		}
//
//		targetSuccess = true
//		log.Info("Detected target successfully registered on SaaS")
//		break
//	}
//	assert.Equal(t, true, targetSuccess, "failed to detect target registered successfully on SaaS")
//
//}
//
//func sendMetrics(envID string, token string, metrics client.PostMetricsJSONRequestBody) (*client.PostMetricsResponse, error) {
//	c := testhelpers.DefaultEvaluationClient(GetStreamURL())
//	resp, err := c.PostMetrics(context.Background(), envID, &client.PostMetricsParams{}, metrics, func(ctx context.Context, req *http.Request) error {
//		req.Header.Set("authorization", fmt.Sprintf("Bearer %s", token))
//		return nil
//	})
//	if err != nil {
//		return nil, err
//	}
//	return client.ParsePostMetricsResponse(resp)
//}
//
//func getFeatureFlag(identifier string, environment string) (*admin.Feature, error) {
//	var temp admin.EnvironmentOptionalQueryParam
//	client := testhelpers.DefaultClient()
//	if len(environment) > 0 {
//		temp = environment
//	}
//	metricsParam := true
//	response, err := client.GetFeatureFlag(context.Background(), identifier, &admin.GetFeatureFlagParams{
//		AccountIdentifier:     GetAccountIdentifier(),
//		OrgIdentifier:         GetOrgIdentifier(),
//		ProjectIdentifier:     GetProjectIdentifier(),
//		EnvironmentIdentifier: &temp,
//		Metrics:               &metricsParam,
//	}, func(ctx context.Context, req *http.Request) error {
//		req.Header.Set("x-api-key", GetUserAccessToken())
//		return nil
//	})
//	if err != nil {
//		log.Error(err)
//		return nil, err
//	}
//
//	flagResponse, err := admin.ParseGetFeatureFlagResponse(response)
//	if err != nil {
//		return nil, err
//	}
//
//	return flagResponse.JSON200, nil
//}
//
//func getFeatureFlagMetrics(identifier string, environment string) (*admin.FeatureMetric, error) {
//	client := testhelpers.DefaultClient()
//
//	response, err := client.GetFeatureMetrics(context.Background(), &admin.GetFeatureMetricsParams{
//		AccountIdentifier:     GetAccountIdentifier(),
//		OrgIdentifier:         GetOrgIdentifier(),
//		ProjectIdentifier:     GetProjectIdentifier(),
//		EnvironmentIdentifier: environment,
//	}, func(ctx context.Context, req *http.Request) error {
//		req.Header.Set("x-api-key", GetUserAccessToken())
//		return nil
//	})
//	if err != nil {
//		log.Error(err)
//		return nil, err
//	}
//
//	metricsResponse, err := admin.ParseGetFeatureMetricsResponse(response)
//	if err != nil {
//		return nil, err
//	}
//	if metricsResponse.StatusCode() != 200 {
//		return nil, fmt.Errorf("metrics non 200 received")
//	}
//	for _, metric := range *metricsResponse.JSON200 {
//		if *metric.Identifier == identifier {
//			return &metric, nil
//		}
//	}
//	log.Warnf("metric not found for %s", identifier)
//	return &admin.FeatureMetric{}, nil
//}
//
//func GetTarget(ctx context.Context, target string, g *admin.GetTargetParams) (*admin.GetTargetResponse, error) {
//	client := testhelpers.DefaultClient()
//	getTarget, _ := client.GetTarget(ctx, target, g, func(ctx context.Context, req *http.Request) error {
//		req.Header.Set("x-api-key", GetUserAccessToken())
//		return nil
//	})
//	return admin.ParseGetTargetResponse(getTarget)
//}
