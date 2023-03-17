package connectivityproxy_test

/*
func TestAction(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	kubeClient := &kubeMocks.Client{}
	kubeClient.On("Clientset").Return(clientset, nil)

	context := &service.ActionContext{
		KubeClient:       kubeClient,
		WorkspaceFactory: nil,
		Context:          nil,
		Logger:           logger.NewLogger(true),
		ChartProvider:    nil,
		Task: &reconciler.Task{
			Component:     "test-component",
			Configuration: make(map[string]interface{}),
		},
	}

	loader := &connectivityproxymocks.Loader{}
	commands := &connectivityproxymocks.Commands{}
	binding := &unstructured.Unstructured{}
	secret := &v1.Secret{}
	statefulset := &v1apps.StatefulSet{}

	action := connectivityproxy.CustomAction{
		Name:     "test-name",
		Loader:   loader,
		Commands: commands,
	}

	commands.On("PopulateConfigs", context, secret).Return(nil)

	t.Run("Should install app if binding exists and app is missing - operator", func(t *testing.T) {
		kubeClient.On("GetStatefulSet", context.Context, "test-component", "").
			Return(nil, nil)
		kubeClient.On("GetHost").
			Return("tmp host")

		loader.On("FindBindingOperator", context).Return(binding, nil)
		loader.On("FindSecret", context, binding).Return(secret, nil)

		commands.On("CopyResources", context).Return(nil)
		commands.On("InstallOrUpgrade", context, (*v1apps.StatefulSet)(nil)).Return(nil)

		err := action.Run(context)
		require.NoError(t, err)
	})

	t.Run("Should install app if binding exists and app is missing - catalog", func(t *testing.T) {
		kubeClient.On("GetStatefulSet", context.Context, "test-component", "").
			Return(nil, nil)
		loader.On("FindBindingOperator", context).Return(nil, nil)
		loader.On("FindBindingCatalog", context).Return(binding, nil)
		loader.On("FindSecret", context, binding).Return(secret, nil)

		commands.On("CopyResources", context).Return(nil)
		commands.On("InstallOrUpgrade", context, (*v1apps.StatefulSet)(nil)).Return(nil)

		err := action.Run(context)
		require.NoError(t, err)
	})

	t.Run("Should remove app if binding is missing and app is existing", func(t *testing.T) {
		kubeClient.On("GetStatefulSet", context.Context, "test-component", "").
			Return(statefulset, nil)
		loader.On("FindBindingOperator", context).Return(nil, nil)
		loader.On("FindBindingCatalog", context).Return(nil, nil)

		commands.On("Remove", context).Return(nil)

		err := action.Run(context)
		require.NoError(t, err)
	})

	t.Run("Should do nothing if binding and app exists ", func(t *testing.T) {
		kubeClient.On("GetStatefulSet", context.Context, "test-component", "").
			Return(statefulset, nil)
		loader.On("FindBindingOperator", context).Return(binding, nil)

		err := action.Run(context)
		require.NoError(t, err)
	})

	t.Run("Should do nothing if binding and app missing ", func(t *testing.T) {
		kubeClient.On("GetStatefulSet", context.Context, "test-component", "").
			Return(statefulset, nil)
		loader.On("FindBindingOperator", context).Return(nil, nil)
		loader.On("FindBindingCatalog", context).Return(nil, nil)

		err := action.Run(context)
		require.NoError(t, err)
	})

	t.Run("Should install when secret not found", func(t *testing.T) {
		kubeClient.On("GetStatefulSet", context.Context, "test-component", "").
			Return(nil, nil)

		loader.On("FindBindingOperator", context).Return(binding, nil)

		loader.On("FindSecret", context, binding).
			Return(nil, nil)

		commands.On("CopyResources", context).Return(nil)
		commands.On("InstallOrUpgrade", context, (*v1apps.StatefulSet)(nil)).Return(nil)

		err := action.Run(context)
		require.NoError(t, err)
	})
}
*/
