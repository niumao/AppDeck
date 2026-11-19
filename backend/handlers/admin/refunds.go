package admin

import (
	"appdeck/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

type RefundsHandler struct {
	service *services.RefundService
}

func NewRefundsHandler() *RefundsHandler {
	return &RefundsHandler{service: services.NewRefundService()}
}

// PreviewRefund handles GET /admin/orders/:id/refund-preview
func (h *RefundsHandler) PreviewRefund(c *gin.Context) {
	orderID := c.Param("id")

	preview, err := h.service.PreviewRefund(orderID)
	if err != nil {
		if err.Error() == "order is not paid" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "Order is not paid",
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Order not found",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    preview,
	})
}

// ProcessRefund handles POST /admin/orders/:id/refund
func (h *RefundsHandler) ProcessRefund(c *gin.Context) {
	orderID := c.Param("id")

	result, err := h.service.ProcessRefund(orderID)
	if err != nil {
		if err.Error() == "no refundable amount" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "No refundable amount",
				"data":    nil,
			})
			return
		}
		if err.Error() == "order is not paid" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "Order is not paid",
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to process refund: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}
