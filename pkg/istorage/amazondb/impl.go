/*
 * Copyright (c) 2024-present unTill Pro, Ltd.
 * @author Alisher Nurmanov
 */

package amazondb

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/coreutils/utils"
	"github.com/voedger/voedger/pkg/istorage"
)

func (d implIAppStorageFactory) AppStorage(appName istorage.SafeAppName) (storage istorage.IAppStorage, err error) {
	cfg, err := newAwsCfg(d.params)
	if err != nil {
		return nil, err
	}

	keySpace := appName.String()
	client := getClient(cfg)

	exist, err := doesTableExist(keySpace, client)
	if err != nil {
		return nil, err
	}

	if !exist {
		return nil, istorage.ErrStorageDoesNotExist
	}

	return &implIAppStorage{client: client, keySpace: dynamoDBTableName(keySpace), iTime: d.iTime}, nil
}

func (d implIAppStorageFactory) Init(appName istorage.SafeAppName) error {
	cfg, err := newAwsCfg(d.params)
	if err != nil {
		return err
	}

	keySpace := appName.String()
	client := getClient(cfg)
	if err := newTableExistsWaiter(keySpace, client); err != nil {
		var awsErr *types.ResourceInUseException
		if errors.As(err, &awsErr) {
			return istorage.ErrStorageAlreadyExists
		}
		return err
	}

	return nil
}

func (d implIAppStorageFactory) Time() coreutils.ITime {
	return d.iTime
}

func (d implIAppStorageFactory) StopGoroutines() { return }

func (s *implIAppStorage) InsertIfNotExists(pKey []byte, cCols []byte, value []byte, ttlSeconds int) (ok bool, err error) {
	found := false
	response, err := s.getItem(pKey, cCols, ttlSeconds > 0)
	if err != nil {
		return false, err
	}

	if response.Item != nil {
		found = true
	}

	if found && !isExpired(response.Item[expireAtAttributeName], s.iTime.Now()) {
		return false, nil
	}

	err = s.put(pKey, cCols, value, ttlSeconds)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (s *implIAppStorage) CompareAndSwap(pKey []byte, cCols []byte, oldValue, newValue []byte, ttlSeconds int) (ok bool, err error) {
	response, err := s.getItem(pKey, cCols, true)
	if err != nil {
		return false, err
	}

	if response.Item == nil {
		return false, nil
	}

	value := response.Item[valueAttributeName].(*types.AttributeValueMemberB).Value
	if !bytes.Equal(value, oldValue) {
		return false, nil
	}

	if isExpired(response.Item[expireAtAttributeName], s.iTime.Now()) {
		return false, nil
	}

	err = s.put(pKey, cCols, newValue, ttlSeconds)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (s *implIAppStorage) CompareAndDelete(pKey []byte, cCols []byte, expectedValue []byte) (ok bool, err error) {
	response, err := s.getItem(pKey, cCols, true)
	if err != nil {
		return false, err
	}

	if response.Item == nil {
		return false, nil
	}

	value := response.Item[valueAttributeName].(*types.AttributeValueMemberB).Value
	if !bytes.Equal(value, expectedValue) {
		return false, nil
	}

	if isExpired(response.Item[expireAtAttributeName], s.iTime.Now()) {
		return false, nil
	}

	_, err = s.client.DeleteItem(context.Background(), &dynamodb.DeleteItemInput{
		TableName: aws.String(s.keySpace),
		Key: map[string]types.AttributeValue{
			partitionKeyAttributeName: &types.AttributeValueMemberB{
				Value: pKey,
			},
			sortKeyAttributeName: &types.AttributeValueMemberB{
				Value: prefixZero(cCols),
			},
		},
	})
	if err != nil {
		return false, err
	}

	return true, nil
}

func (s *implIAppStorage) TTLGet(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error) {
	return s.get(pKey, cCols, data, true)
}

func (s *implIAppStorage) TTLRead(ctx context.Context, pKey []byte, startCCols, finishCCols []byte, cb istorage.ReadCallback) (err error) {
	return s.read(ctx, pKey, startCCols, finishCCols, cb, true)
}

func (s *implIAppStorage) Put(pKey []byte, cCols []byte, value []byte) (err error) {
	return s.put(pKey, cCols, value, 0)
}

func (s *implIAppStorage) PutBatch(items []istorage.BatchItem) (err error) {
	writeRequests := make([]types.WriteRequest, len(items))
	for i, item := range items {
		writeRequests[i].PutRequest = &types.PutRequest{
			Item: map[string]types.AttributeValue{
				partitionKeyAttributeName: &types.AttributeValueMemberB{
					Value: item.PKey,
				},
				sortKeyAttributeName: &types.AttributeValueMemberB{
					Value: prefixZero(item.CCols),
				},
				valueAttributeName: &types.AttributeValueMemberB{
					Value: item.Value,
				},
			},
		}
	}
	params := dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			s.keySpace: writeRequests,
		},
	}
	_, err = s.client.BatchWriteItem(context.Background(), &params)

	return err
}

func (s *implIAppStorage) Get(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error) {
	return s.get(pKey, cCols, data, false)
}

func (s *implIAppStorage) GetBatch(pKey []byte, items []istorage.GetBatchItem) error {
	// Reset data slices for all items
	for i, item := range items {
		*item.Data = (*item.Data)[:0]
		items[i].Ok = false
	}
	tableName := s.keySpace

	cColToIndex := make(map[string][]int)
	keyList := make([]map[string]types.AttributeValue, 0)
	uniqueCCols := make(map[string]struct{})
	for i, item := range items {
		patchedCCols := prefixZero(item.CCols)
		strPatchedCCols := string(patchedCCols)
		cColToIndex[strPatchedCCols] = append(cColToIndex[strPatchedCCols], i)
		if _, ok := uniqueCCols[strPatchedCCols]; ok {
			continue
		}
		uniqueCCols[strPatchedCCols] = struct{}{}

		keyList = append(keyList, map[string]types.AttributeValue{
			partitionKeyAttributeName: &types.AttributeValueMemberB{
				Value: pKey,
			},
			sortKeyAttributeName: &types.AttributeValueMemberB{
				Value: patchedCCols,
			},
		})
	}

	params := dynamodb.BatchGetItemInput{
		RequestItems: map[string]types.KeysAndAttributes{
			tableName: {
				Keys:                     keyList,
				ProjectionExpression:     aws.String(sortKeyAttributeName + ", #v"),
				ExpressionAttributeNames: map[string]string{"#v": valueAttributeName},
			},
		},
	}

	result, err := s.client.BatchGetItem(context.Background(), &params)
	if err != nil {
		return err
	}

	if len(result.Responses) > 0 {
		for _, item := range result.Responses[tableName] {
			indexList := cColToIndex[string(item[sortKeyAttributeName].(*types.AttributeValueMemberB).Value)]
			for _, index := range indexList {
				items[index].Ok = true
				*items[index].Data = item[valueAttributeName].(*types.AttributeValueMemberB).Value
			}
		}
	}

	return nil
}

func (s *implIAppStorage) Read(ctx context.Context, pKey []byte, startCCols, finishCCols []byte, cb istorage.ReadCallback) (err error) {
	return s.read(ctx, pKey, startCCols, finishCCols, cb, false)
}

func (s *implIAppStorage) get(pKey []byte, cCols []byte, data *[]byte, checkTtl bool) (ok bool, err error) {
	response, err := s.getItem(pKey, cCols, checkTtl)
	if err != nil {
		return false, err
	}

	if response.Item == nil {
		return false, nil
	}

	*data = (*data)[:0] // Reset the data slice
	if checkTtl && isExpired(response.Item[expireAtAttributeName], s.iTime.Now()) {
		return false, nil
	}

	// Extract the value attribute from the response
	valueAttribute := response.Item[valueAttributeName]
	*data = valueAttribute.(*types.AttributeValueMemberB).Value

	return true, nil
}

func (s *implIAppStorage) getItem(pKey []byte, cCols []byte, getTtl bool) (*dynamodb.GetItemOutput, error) {
	// arranging request payload
	params := dynamodb.GetItemInput{
		TableName: aws.String(s.keySpace),
		Key: map[string]types.AttributeValue{
			partitionKeyAttributeName: &types.AttributeValueMemberB{
				Value: pKey,
			},
			sortKeyAttributeName: &types.AttributeValueMemberB{
				Value: prefixZero(cCols),
			},
		},
		ProjectionExpression:     aws.String(sortKeyAttributeName + ", #v"),
		ExpressionAttributeNames: map[string]string{"#v": valueAttributeName},
	}

	if getTtl {
		params.ProjectionExpression = aws.String(sortKeyAttributeName + ", #v, #e")
		params.ExpressionAttributeNames["#e"] = expireAtAttributeName
	}

	// making request to DynamoDB
	// GetItem method returns response (pointer to GetItemOutput struct) and error
	return s.client.GetItem(context.Background(), &params)
}

func (s *implIAppStorage) put(pKey []byte, cCols []byte, value []byte, ttlSeconds int) (err error) {
	putItemParams := dynamodb.PutItemInput{
		TableName: aws.String(s.keySpace),
		Item: map[string]types.AttributeValue{
			partitionKeyAttributeName: &types.AttributeValueMemberB{
				Value: pKey,
			},
			sortKeyAttributeName: &types.AttributeValueMemberB{
				Value: prefixZero(cCols),
			},
			valueAttributeName: &types.AttributeValueMemberB{
				Value: value,
			},
		},
	}

	if ttlSeconds > 0 {
		putItemParams.Item[expireAtAttributeName] = &types.AttributeValueMemberN{
			Value: strconv.FormatInt(s.iTime.Now().Add(time.Duration(ttlSeconds)*time.Second).Unix(), utils.DecimalBase),
		}
	}
	_, err = s.client.PutItem(context.Background(), &putItemParams)

	return err
}

func (s *implIAppStorage) read(ctx context.Context, pKey []byte, startCCols, finishCCols []byte, cb istorage.ReadCallback, checkTtl bool) (err error) {
	if (len(startCCols) > 0) && (len(finishCCols) > 0) && (bytes.Compare(startCCols, finishCCols) >= 0) {
		return nil // absurd range
	}

	keyConditions := map[string]types.Condition{
		partitionKeyAttributeName: {
			ComparisonOperator: types.ComparisonOperatorEq,
			AttributeValueList: []types.AttributeValue{
				&types.AttributeValueMemberB{
					Value: pKey,
				},
			},
		},
	}

	rightBorder := &types.AttributeValueMemberB{
		Value: prefixZero(finishCCols),
	}
	leftBorder := &types.AttributeValueMemberB{
		Value: prefixZero(startCCols),
	}

	switch {
	case len(startCCols) == 0:
		if len(finishCCols) != 0 {
			keyConditions[sortKeyAttributeName] = types.Condition{
				ComparisonOperator: types.ComparisonOperatorLe,
				AttributeValueList: []types.AttributeValue{rightBorder},
			}
		}
	case len(finishCCols) == 0:
		// right-opened range
		keyConditions[sortKeyAttributeName] = types.Condition{
			ComparisonOperator: types.ComparisonOperatorGe,
			AttributeValueList: []types.AttributeValue{leftBorder},
		}
	default:
		// closed range
		keyConditions[sortKeyAttributeName] = types.Condition{
			ComparisonOperator: types.ComparisonOperatorBetween,
			AttributeValueList: []types.AttributeValue{leftBorder, rightBorder},
		}
	}

	params := dynamodb.QueryInput{
		TableName:                aws.String(s.keySpace),
		ProjectionExpression:     aws.String(sortKeyAttributeName + ", #v"),
		ExpressionAttributeNames: map[string]string{"#v": valueAttributeName},
		KeyConditions:            keyConditions,
	}

	if checkTtl {
		params.ProjectionExpression = aws.String(sortKeyAttributeName + ", #v, #e")
		params.ExpressionAttributeNames["#e"] = expireAtAttributeName
	}

	result, err := s.client.Query(ctx, &params)
	if err != nil {
		return err
	}

	now := s.iTime.Now()
	if len(result.Items) > 0 {
		for _, item := range result.Items {
			if ctx.Err() != nil {
				return nil // TCK contract
			}

			if checkTtl && isExpired(item[expireAtAttributeName], now) {
				continue
			}

			if err := cb(
				unprefixZero(item[sortKeyAttributeName].(*types.AttributeValueMemberB).Value),
				item[valueAttributeName].(*types.AttributeValueMemberB).Value,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func getClient(cfg aws.Config) *dynamodb.Client {
	return dynamodb.NewFromConfig(cfg)
}

func newAwsCfg(params DynamoDBParams) (aws.Config, error) {
	return config.LoadDefaultConfig(context.Background(),
		config.WithBaseEndpoint(params.EndpointURL),
		config.WithRegion(params.Region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				params.AccessKeyID,
				params.SecretAccessKey,
				params.SessionToken,
			),
		),
	)
}

func newTableExistsWaiter(name string, client *dynamodb.Client) error {
	ctx := context.Background()
	createTableInput := &dynamodb.CreateTableInput{
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String(partitionKeyAttributeName),
				AttributeType: types.ScalarAttributeTypeB,
			},
			{
				AttributeName: aws.String(sortKeyAttributeName),
				AttributeType: types.ScalarAttributeTypeB,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String(partitionKeyAttributeName),
				KeyType:       types.KeyTypeHash,
			},
			{
				AttributeName: aws.String(sortKeyAttributeName),
				KeyType:       types.KeyTypeRange,
			},
		},
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(defaultRCU),
			WriteCapacityUnits: aws.Int64(defaultWCU),
		},
		TableName: aws.String(dynamoDBTableName(name)),
	}

	if _, err := client.CreateTable(ctx, createTableInput); err != nil {
		return err
	}

	// Enable ttl for the table
	input := &dynamodb.UpdateTimeToLiveInput{
		TableName: aws.String(dynamoDBTableName(name)),
		TimeToLiveSpecification: &types.TimeToLiveSpecification{
			AttributeName: aws.String(expireAtAttributeName),
			Enabled:       aws.Bool(true),
		},
	}

	_, err := client.UpdateTimeToLive(ctx, input)

	return err
}

func isExpired(expireAtValue types.AttributeValue, now time.Time) bool {
	if expireAtValue == nil {
		return false
	}

	if len(expireAtValue.(*types.AttributeValueMemberN).Value) == 0 {
		return false
	}

	expireAtInSeconds, err := strconv.ParseInt(expireAtValue.(*types.AttributeValueMemberN).Value, utils.DecimalBase, utils.BitSize64)
	if err != nil {
		return false
	}
	expireAt := time.Unix(expireAtInSeconds, 0)

	return !expireAt.After(now)
}

func doesTableExist(name string, client *dynamodb.Client) (bool, error) {
	describeTableInput := &dynamodb.DescribeTableInput{
		TableName: aws.String(dynamoDBTableName(name)),
	}

	if _, err := client.DescribeTable(context.Background(), describeTableInput); err != nil {
		// Check if the error indicates that the table doesn't exist
		var resourceNotFoundException *types.ResourceNotFoundException
		if errors.As(err, &resourceNotFoundException) {
			return false, nil
		}
		// Any other error
		return false, err
	}
	// Table exists
	return true, nil
}

func dynamoDBTableName(name string) string {
	return name + ".values"
}

// prefixZero is a workaround for DynamoDB's limitation on empty byte slices in SortKey
// https://aws.amazon.com/ru/about-aws/whats-new/2020/05/amazon-dynamodb-now-supports-empty-values-for-non-key-string-and-binary-attributes-in-dynamodb-tables/
func prefixZero(value []byte) (out []byte) {
	newArr := make([]byte, 1, len(value)+1)
	newArr[0] = 0
	return append(newArr, value...)
}

// unprefixZero is a workaround for DynamoDB's limitation on empty byte slices in SortKey
// https://aws.amazon.com/ru/about-aws/whats-new/2020/05/amazon-dynamodb-now-supports-empty-values-for-non-key-string-and-binary-attributes-in-dynamodb-tables/
func unprefixZero(value []byte) (out []byte) {
	return value[1:]
}
