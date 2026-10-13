package alert

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
)

type Sender interface {
	Send(ctx context.Context, phone, message string) error
}

type LogSender struct {
	Logger *slog.Logger
}

func (l LogSender) Send(_ context.Context, phone, message string) error {
	l.Logger.Info("missed-dose alert", "to", maskPhone(phone), "message", message)
	return nil
}

func maskPhone(phone string) string {
	if len(phone) <= 7 {
		return strings.Repeat("*", len(phone))
	}
	return phone[:3] + strings.Repeat("*", len(phone)-7) + phone[len(phone)-4:]
}

type publisher interface {
	Publish(context.Context, *sns.PublishInput, ...func(*sns.Options)) (*sns.PublishOutput, error)
}

type SNSSender struct {
	client publisher
}

func NewSNSSender(ctx context.Context) (*SNSSender, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	if cfg.Region == "" {
		return nil, errors.New("Amazon SNS needs an AWS region; set AWS_REGION")
	}
	return &SNSSender{client: sns.NewFromConfig(cfg)}, nil
}

func (s *SNSSender) Send(ctx context.Context, phone, message string) error {
	_, err := s.client.Publish(ctx, &sns.PublishInput{
		PhoneNumber: aws.String(phone),
		Message:     aws.String(message),
		MessageAttributes: map[string]types.MessageAttributeValue{
			"AWS.SNS.SMS.SMSType": {DataType: aws.String("String"), StringValue: aws.String("Transactional")},
		},
	})
	return err
}
