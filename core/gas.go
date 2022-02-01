package core

import (
	"github.com/golang/protobuf/proto"
	pb "github.com/guyu96/go-timbre/protobuf"
)

//TODO:- remove or keep cpuusage

//ResourceUsage is the structure for calculating tx payments according to the computation and network bandwidth used
type ResourceUsage struct {
	cpuUsage int64
	netUsage int64
}

//ResourcePrice contains the price of the resources
type ResourcePrice struct {
	CPUPrice float64
	NetPrice float64
}

//TestRP is the pricing for testing
var TestRP *ResourcePrice = &ResourcePrice{
	CPUPrice: 1,
	NetPrice: 0.2,
}

//NewResouceUsage returns new object of the ResourceUsage
func NewResouceUsage(cpu, net int64) *ResourceUsage {
	return &ResourceUsage{
		cpuUsage: cpu,
		netUsage: net,
	}
}

//GetCpuUsage returns the CPU usage
func (r *ResourceUsage) GetCpuUsage() int64 {
	return r.cpuUsage
}

//GetNetUsage returns the network bandwidth consumed
func (r *ResourceUsage) GetNetUsage() int64 {
	return r.netUsage
}

//SetCpuUsage sets the cpu usage
func (r *ResourceUsage) SetCpuUsage(cpu int64) {
	r.cpuUsage = cpu
}

//SetNetUsage sets the netUsage
func (r *ResourceUsage) SetNetUsage(net int64) {
	r.netUsage = net
}

//CalculateTotalCost calculates the total cost of using the cpu and net resources
func (r *ResourceUsage) CalculateTotalCost(price *ResourcePrice) int64 {
	// CpuCost := uint64(float64(r.cpuUsage) * price.CPUPrice)  //For now not using the CPU cost
	NetCost := int64(float64(r.netUsage) * price.NetPrice)

	totalCost := NetCost
	return totalCost
}

//ResourcesRequired gives the resources utilized
func ResourcesRequired(tx interface{}) *ResourceUsage {
	//THis functon sets the absolute CPUusage values for different types of objects to be added to the ledger
	switch obj := tx.(type) {

	case *pb.Vote:
		voteBytes, _ := proto.Marshal(obj)
		return &ResourceUsage{
			cpuUsage: 10,
			netUsage: int64(len(voteBytes)),
		}
	case *pb.Transaction:
		transBytes, _ := proto.Marshal(obj)
		switch txType := obj.Type; txType {
		case TxBecomeCandidate: //For Becoming candidate
			return &ResourceUsage{
				cpuUsage: 13,
				netUsage: int64(len(transBytes)),
			}
		case TxQuitCandidate: //For quiting candidate
			return &ResourceUsage{
				cpuUsage: 11,
				netUsage: int64(len(transBytes)),
			}
		case TxTransfer:
			return &ResourceUsage{ // For the retrieval transactions
				cpuUsage: 10,
				netUsage: int64(len(transBytes)),
			}
		case TxVote:
			return &ResourceUsage{ // For the retrieval transactions
				cpuUsage: 10,
				netUsage: int64(len(transBytes)),
			}
		case TxPodf:
			return &ResourceUsage{ // For the retrieval transactions
				cpuUsage: 10,
				netUsage: int64(len(transBytes)),
			}

		default:
			return &ResourceUsage{ //For now treating it as a retrieval transaction
				cpuUsage: 10,
				netUsage: int64(len(transBytes)),
			}
		}

	case *pb.Deal:
		dealBytes, _ := proto.Marshal(obj)
		return &ResourceUsage{
			cpuUsage: 15,
			netUsage: int64(len(dealBytes)),
		}
	case *pb.ChPrPair:
		proofBytes, _ := proto.Marshal(obj)
		return &ResourceUsage{
			cpuUsage: 12,
			netUsage: int64(len(proofBytes)),
		}
	case *pb.SignedPostInfo:
		postBytes, _ := proto.Marshal(obj)
		return &ResourceUsage{
			cpuUsage: 12,
			netUsage: (int64(len(postBytes)) + int64(obj.Info.GetMetadata().GetContentSize())),
		}
	case *pb.PoDF:
		podfBytes, _ := proto.Marshal(obj)
		return &ResourceUsage{
			cpuUsage: 10,
			netUsage: int64(len(podfBytes)),
		}

	default:
		// log.Info().Msgf("Invalid type found. Can't find the resources required for processing ", obj)
		return nil
	}
}

//GetTxFee returns the transaction fee according to the test resource price
func GetTxFee(tx interface{}) int64 {
	resourceEstimate := ResourcesRequired(tx)
	if resourceEstimate == nil {
		return 0
	}
	txFee := resourceEstimate.CalculateTotalCost(TestRP)
	return txFee
}
