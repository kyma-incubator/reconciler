package db

type ContainerRuntime interface {
	ContainerBootstrap
	ConnectionFactory
}
