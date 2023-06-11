package utils

import (
	"context"
	"strings"

	. "0442a403.hse.ru/mafia/engine/common"

	"google.golang.org/grpc/peer"
)

func ParseIp(ctx context.Context) Ip {
	p, _ := peer.FromContext(ctx)
	return Ip(strings.Split(p.Addr.String(), ":")[0])
}
