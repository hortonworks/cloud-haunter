package aws

import (
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeS3 is the state shared by the regional mockS3Clients: buckets, their location, tags and objects.
// Every call is recorded as "region:Operation:bucket[/key]".
type fakeS3 struct {
	created     time.Time
	locations   map[string]*string // bucket -> GetBucketLocation constraint (nil means us-east-1)
	tags        map[string][]*s3.Tag
	tagErrs     map[string]error
	objects     map[string][]*s3.Object
	listObjErrs map[string]error

	mu    sync.Mutex
	calls []string
}

func (f *fakeS3) record(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
}

func (f *fakeS3) recordedCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	calls := append([]string(nil), f.calls...)
	sort.Strings(calls)
	return calls
}

func (f *fakeS3) clients(regions ...string) map[string]s3Client {
	clients := map[string]s3Client{}
	for _, region := range regions {
		clients[region] = mockS3Client{region: region, fake: f}
	}
	return clients
}

type mockS3Client struct {
	region string
	fake   *fakeS3
}

func (c mockS3Client) ListBuckets(*s3.ListBucketsInput) (*s3.ListBucketsOutput, error) {
	var names []string
	for name := range c.fake.locations {
		names = append(names, name)
	}
	sort.Strings(names)
	var buckets []*s3.Bucket
	for _, name := range names {
		buckets = append(buckets, &s3.Bucket{Name: aws.String(name), CreationDate: aws.Time(c.fake.created)})
	}
	return &s3.ListBucketsOutput{Buckets: buckets}, nil
}

func (c mockS3Client) GetBucketLocation(input *s3.GetBucketLocationInput) (*s3.GetBucketLocationOutput, error) {
	return &s3.GetBucketLocationOutput{LocationConstraint: c.fake.locations[*input.Bucket]}, nil
}

func (c mockS3Client) GetBucketTagging(input *s3.GetBucketTaggingInput) (*s3.GetBucketTaggingOutput, error) {
	c.fake.record(c.region + ":GetBucketTagging:" + *input.Bucket)
	if err := c.fake.tagErrs[*input.Bucket]; err != nil {
		return nil, err
	}
	return &s3.GetBucketTaggingOutput{TagSet: c.fake.tags[*input.Bucket]}, nil
}

func (c mockS3Client) ListObjectsV2(input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
	c.fake.record(c.region + ":ListObjectsV2:" + *input.Bucket)
	if err := c.fake.listObjErrs[*input.Bucket]; err != nil {
		return nil, err
	}
	return &s3.ListObjectsV2Output{Contents: c.fake.objects[*input.Bucket], IsTruncated: aws.Bool(false)}, nil
}

func (c mockS3Client) DeleteObject(input *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error) {
	c.fake.record(c.region + ":DeleteObject:" + *input.Bucket + "/" + *input.Key)
	return nil, nil
}

func (c mockS3Client) DeleteBucket(input *s3.DeleteBucketInput) (*s3.DeleteBucketOutput, error) {
	c.fake.record(c.region + ":DeleteBucket:" + *input.Bucket)
	return nil, nil
}

func storageByName(storages []*types.Storage) map[string]*types.Storage {
	byName := map[string]*types.Storage{}
	for _, s := range storages {
		byName[s.Name] = s
	}
	return byName
}

func TestGetStoragesFetchesTagsAndOwner(t *testing.T) {
	fake := &fakeS3{
		created: NOW,
		locations: map[string]*string{
			"ignored-bucket":  nil,
			"untagged-bucket": aws.String("EU"),
			"eu-bucket":       aws.String("eu-central-1"),
		},
		tags: map[string][]*s3.Tag{
			"ignored-bucket": {
				{Key: aws.String(config.DefaultIgnoreLabel), Value: aws.String("true")},
				{Key: aws.String(config.DefaultOwnerLabel), Value: aws.String("alice")},
			},
			"eu-bucket": {{Key: aws.String(config.DefaultOwnerLabel), Value: aws.String("bob")}},
		},
		tagErrs: map[string]error{"untagged-bucket": awserr.New("NoSuchTagSet", "The TagSet does not exist", nil)},
	}

	storages, err := getStorages(testCfg(), fake.clients(DEFAULT_REGION, "eu-west-1", "eu-central-1"))

	require.NoError(t, err)
	byName := storageByName(storages)
	require.Len(t, byName, 3)

	assert.Equal(t, DEFAULT_REGION, byName["ignored-bucket"].Region)
	assert.Equal(t, types.Tags{config.DefaultIgnoreLabel: "true", config.DefaultOwnerLabel: "alice"}, byName["ignored-bucket"].Tags)
	assert.Equal(t, "alice", byName["ignored-bucket"].Owner)

	assert.Equal(t, "eu-west-1", byName["untagged-bucket"].Region, "legacy EU location is normalized")
	assert.Empty(t, byName["untagged-bucket"].Tags)
	assert.Empty(t, byName["untagged-bucket"].Owner)

	assert.Equal(t, "bob", byName["eu-bucket"].Owner)

	assert.Equal(t, []string{
		"eu-central-1:GetBucketTagging:eu-bucket",
		"eu-west-1:GetBucketTagging:untagged-bucket",
		DEFAULT_REGION + ":GetBucketTagging:ignored-bucket",
	}, fake.recordedCalls(), "tags are fetched with the client of the bucket's region")
}

func TestGetStoragesFailsWhenTagsCannotBeFetched(t *testing.T) {
	fake := &fakeS3{
		created:   NOW,
		locations: map[string]*string{"bucket": nil},
		tagErrs:   map[string]error{"bucket": awserr.New("AccessDenied", "Access Denied", nil)},
	}

	storages, err := getStorages(testCfg(), fake.clients(DEFAULT_REGION))

	assert.Error(t, err)
	assert.Nil(t, storages)
}

func TestGetStoragesSkipsBucketsInRegionsWithoutClient(t *testing.T) {
	fake := &fakeS3{
		created:   NOW,
		locations: map[string]*string{"bucket": nil, "excluded-region-bucket": aws.String("ap-south-1")},
	}

	storages, err := getStorages(testCfg(), fake.clients(DEFAULT_REGION))

	require.NoError(t, err)
	var names []string
	for _, s := range storages {
		names = append(names, s.Name)
	}
	assert.Equal(t, []string{"bucket"}, names)
}

func TestCleanupStorages(t *testing.T) {
	old := NOW.AddDate(0, 0, -10)
	newObjects := func() map[string][]*s3.Object {
		return map[string][]*s3.Object{"bucket": {
			{Key: aws.String("old"), LastModified: aws.Time(old), Size: aws.Int64(1)},
			{Key: aws.String("new"), LastModified: aws.Time(NOW), Size: aws.Int64(1)},
		}}
	}
	cases := []struct {
		name         string
		dryRun       bool
		listObjErr   error
		expected     []string
		expectedErrs int
	}{
		{name: "deletes old objects and the old bucket", expected: []string{
			"region:DeleteBucket:bucket", "region:DeleteObject:bucket/old", "region:ListObjectsV2:bucket"}},
		{name: "dry-run only lists", dryRun: true, expected: []string{"region:ListObjectsV2:bucket"}},
		{name: "listing failure skips the bucket", listObjErr: errors.New("listing failed"), expectedErrs: 1,
			expected: []string{"region:ListObjectsV2:bucket"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeS3{objects: newObjects(), listObjErrs: map[string]error{"bucket": tc.listObjErr}}
			cfg := testCfg()
			cfg.DryRun = tc.dryRun
			storages := []*types.Storage{{CloudType: types.AWS, Name: "bucket", Region: "region", Created: old}}

			errs := cleanupStorages(cfg, fake.clients("region"), types.NewStorageContainer(storages), 5)

			assert.Len(t, errs, tc.expectedErrs)
			assert.Equal(t, tc.expected, fake.recordedCalls())
		})
	}
}
