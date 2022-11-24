package cni

/* func Test_isCNIEnabled(t *testing.T) {
	t.Run("should get the CNI enabled value from istio chart", func(t *testing.T) {
		// given
		branch := "branch"
		istioChart := "istio-cni-enabled"
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{ResourceDir: "../test_files"}, nil)

		// when
		cniEnabled, err := isCniEnabledInChart(factory, branch, istioChart)

		// then
		require.NoError(t, err)
		require.EqualValues(t, true, cniEnabled)
	})
}

func Test_isCNIRolloutRequired(t *testing.T) {
	branch := "branch"
	istioChart := "istio-cni-enabled"
	log := logger.NewLogger(false)
	t.Run("should start proxy reset if CNI config map change the default Helm chart value", func(t *testing.T) {
		// given
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{ResourceDir: "../test_files"}, nil)
		configMapValueString := "false"
		istioCNIConfigMap := &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name:      configMapCNI,
			Namespace: kymaNamespace,
		},
			Data: map[string]string{"enabled": configMapValueString},
		}
		client := fake.NewSimpleClientset(istioCNIConfigMap)

		// when
		proxyRolloutRequired, err := IsRolloutRequired(context.TODO(), client, factory, branch, istioChart, log)

		// then
		require.NoError(t, err)
		require.EqualValues(t, true, proxyRolloutRequired)
	})
	t.Run("should not start proxy reset if CNI config map does not change the default Helm chart value", func(t *testing.T) {
		// given
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{ResourceDir: "../test_files"}, nil)
		configMapValueString := "true"
		istioCNIConfigMap := &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name:      configMapCNI,
			Namespace: kymaNamespace,
		},
			Data: map[string]string{"enabled": configMapValueString},
		}
		client := fake.NewSimpleClientset(istioCNIConfigMap)

		// when
		proxyRolloutRequired, err := IsRolloutRequired(context.TODO(), client, factory, branch, istioChart, log)

		// then
		require.NoError(t, err)
		require.EqualValues(t, false, proxyRolloutRequired)
	})
}
*/
