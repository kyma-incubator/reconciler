package cmd

import (
	"os"
	"sort"
	"time"

	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/spf13/cobra"
)

func NewCmd(o *cli.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "bucket",
		Aliases: []string{"buckets", "bu"},
		Short:   "Get configuration buckets.",
		Long:    `List configuration buckets or get a bucket inclusive its containing configuration entries.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(o, args)
		},
	}
	return cmd
}

func Run(o *cli.Options, bucketFilter []string) error {
	allBuckets, err := o.Registry.KVRepository().Buckets()
	if err != nil {
		return err
	}

	//if filter is defined, show buckets with content
	if len(bucketFilter) > 0 {
		filteredBuckets := filterBuckets(allBuckets, bucketFilter)
		return renderBucketsWithValues(o, filteredBuckets)
	}

	//otherwise show just the bucket itself
	return renderBuckets(o, allBuckets)
}

func filterBuckets(allBuckets []*model.BucketEntity, bucketFilter []string) []*model.BucketEntity {
	//to improve speed, use map from bucket-name to bucket-entity
	bucketByName := make(map[string]*model.BucketEntity, len(bucketFilter))
	for _, bucket := range allBuckets {
		bucketByName[bucket.Bucket] = bucket
	}

	//filter buckets
	filteredBuckets := []*model.BucketEntity{}
	sort.Strings(bucketFilter) //ensure the filtered buckets are added to result in alphabetical order
	for _, filter := range bucketFilter {
		if bucket, ok := bucketByName[filter]; ok {
			filteredBuckets = append(filteredBuckets, bucket)
		}
	}
	return filteredBuckets
}

func renderBuckets(o *cli.Options, buckets []*model.BucketEntity) error {
	formatter, err := cli.NewOutputFormatter(o.OutputFormat)
	if err != nil {
		return err
	}

	if err := formatter.Header("Bucket", "Created by", "Created at (UTC)"); err != nil {
		return err
	}
	for _, bucket := range buckets {
		if err := formatter.AddRow(bucket.Bucket, bucket.Username, bucket.Created.Format(time.RFC822Z)); err != nil {
			return err
		}
	}
	return formatter.Output(os.Stdout)
}

func renderBucketsWithValues(o *cli.Options, buckets []*model.BucketEntity) error {
	formatter, err := cli.NewOutputFormatter(o.OutputFormat)
	if err != nil {
		return err
	}

	if err := formatter.Header("Bucket", "Created by", "Created at (UTC)", "Content"); err != nil {
		return err
	}
	for _, bucket := range buckets {
		values, err := o.Registry.KVRepository().ValuesByBucket(bucket.Bucket)
		if err != nil {
			return err
		}
		kvPairs := make(map[string]interface{}, len(values))
		for _, value := range values {
			kvPairs[value.Key] = value.Value
		}
		if err := formatter.AddRow(bucket.Bucket, bucket.Username, bucket.Created.Format(time.RFC822Z), kvPairs); err != nil {
			return err
		}
	}
	return formatter.Output(os.Stdout)
}
