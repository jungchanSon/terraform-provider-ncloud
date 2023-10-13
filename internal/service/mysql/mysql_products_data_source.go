package mysql

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/NaverCloudPlatform/ncloud-sdk-go-v2/services/vmysql"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/terraform-providers/terraform-provider-ncloud/internal/common"
	"github.com/terraform-providers/terraform-provider-ncloud/internal/conn"
	"time"
)

var (
	_ datasource.DataSource              = &mysqlProductsDataSource{}
	_ datasource.DataSourceWithConfigure = &mysqlProductsDataSource{}
)

func NewMysqlProductsDataSource() datasource.DataSource {
	return &mysqlProductsDataSource{}
}

type mysqlProductsDataSource struct {
	config *conn.ProviderConfig
}

func (m *mysqlProductsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	config, ok := req.ProviderData.(*conn.ProviderConfig)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *ProviderConfig, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	m.config = config
}

func (m *mysqlProductsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mysql_products"
}

func (m *mysqlProductsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"cloud_mysql_image_product_code": schema.StringAttribute{
				Required: true,
			},
			"product_code": schema.StringAttribute{
				Optional: true,
			},
			"exclusion_product_code": schema.StringAttribute{
				Optional: true,
			},
			"output_file": schema.StringAttribute{
				Optional: true,
			},
			"product_list": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"product_code": schema.StringAttribute{
							Computed: true,
						},
						"product_name": schema.StringAttribute{
							Computed: true,
						},
						"product_type": schema.StringAttribute{
							Computed: true,
						},
						"product_description": schema.StringAttribute{
							Computed: true,
						},
						"infra_resource_type": schema.StringAttribute{
							Computed: true,
						},
						"cpu_count": schema.Int64Attribute{
							Computed: true,
						},
						"memory_size": schema.Int64Attribute{
							Computed: true,
						},
						"disk_type": schema.StringAttribute{
							Computed: true,
						},
					},
				},
				Computed: true,
			},
		},

		Blocks: map[string]schema.Block{
			"filter": common.DataSourceFiltersBlock(),
		},
	}
}

func (m *mysqlProductsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data mysqlProductList
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	reqParams := &vmysql.GetCloudMysqlProductListRequest{
		RegionCode:                 &m.config.RegionCode,
		CloudMysqlImageProductCode: data.CloudMysqlImageProductCode.ValueStringPointer(),
	}

	if !data.ProductCode.IsNull() && !data.ProductCode.IsUnknown() {
		reqParams.ProductCode = data.ProductCode.ValueStringPointer()
	}

	if !data.ExclusionProductCode.IsNull() && !data.ExclusionProductCode.IsUnknown() {
		reqParams.ExclusionProductCode = data.ExclusionProductCode.ValueStringPointer()
	}

	tflog.Info(ctx, "GetMysqlProductsList", map[string]any{
		"reqParams": common.MarshalUncheckedString(reqParams),
	})

	mysqlProductResp, err := m.config.Client.Vmysql.V2Api.GetCloudMysqlProductList(reqParams)
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError(
			"GetMysqlProductList",
			fmt.Sprintf("error: %s, reqParams: %s", err.Error(), common.MarshalUncheckedString(reqParams)),
		)
		resp.Diagnostics.Append(diags...)
		return
	}

	tflog.Info(ctx, "GetMysqlProductList response", map[string]any{
		"mysqlProductResponse": common.MarshalUncheckedString(mysqlProductResp),
	})

	mysqlProductList := flattenMysqlProduct(ctx, mysqlProductResp.ProductList)
	fillteredList := common.FilterModels(ctx, data.Filters, mysqlProductList)

	data.refreshFromOutput(ctx, fillteredList)

	var mysqlImagesToConvert = []mysqlProductsToJsonConvert{}
	for _, image := range data.ProductList.Elements() {
		imageJasn := mysqlProductsToJsonConvert{}
		json.Unmarshal([]byte(image.String()), &imageJasn)
		mysqlImagesToConvert = append(mysqlImagesToConvert, imageJasn)
	}

	if !data.OutputFile.IsNull() && data.OutputFile.String() != "" {
		outputPath := data.OutputFile.ValueString()

		common.WriteToFile(outputPath, mysqlImagesToConvert)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func flattenMysqlProduct(ctx context.Context, list []*vmysql.Product) []*mysqlProductModel {
	var outputs []*mysqlProductModel

	for _, v := range list {
		var output mysqlProductModel
		output.refreshFromOutput(v)

		outputs = append(outputs, &output)
	}
	return outputs
}

func (m *mysqlProductList) refreshFromOutput(ctx context.Context, list []*mysqlProductModel) {
	productListValue, _ := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: mysqlProductModel{}.attrTypes()}, list)
	m.ProductList = productListValue
	m.ID = types.StringValue(time.Now().UTC().String())
}

type mysqlProductList struct {
	ID                         types.String `tfsdk:"id"`
	CloudMysqlImageProductCode types.String `tfsdk:"cloud_mysql_image_product_code"`
	ProductCode                types.String `tfsdk:"product_code"`
	ExclusionProductCode       types.String `tfsdk:"exclusion_product_code"`
	ProductList                types.List   `tfsdk:"product_list"`
	OutputFile                 types.String `tfsdk:"output_file"`
	Filters                    types.Set    `tfsdk:"filter"`
}

type mysqlProductModel struct {
	ProductCode        types.String `tfsdk:"product_code"`
	ProductName        types.String `tfsdk:"product_name"`
	ProductType        types.String `tfsdk:"product_type"`
	ProductDescription types.String `tfsdk:"product_description"`
	InfraResourceType  types.String `tfsdk:"infra_resource_type"`
	CpuCount           types.Int64  `tfsdk:"cpu_count"`
	MemorySize         types.Int64  `tfsdk:"memory_size"`
	DiskType           types.String `tfsdk:"disk_type"`
}
type mysqlProductsToJsonConvert struct {
	ProductCode        string `json:"product_code"`
	ProductName        string `json:"product_name"`
	ProductType        string `json:"product_type"`
	ProductDescription string `json:"product_description"`
	InfraResourceType  string `json:"infra_resource_type"`
	CpuCount           int    `json:"cpu_count"`
	MemorySize         int    `json:"memory_size"`
	DiskType           string `json:"disk_type"`
}

func (m mysqlProductModel) attrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"product_code":        types.StringType,
		"product_name":        types.StringType,
		"product_type":        types.StringType,
		"infra_resource_type": types.StringType,
		"cpu_count":           types.Int64Type,
		"memory_size":         types.Int64Type,
		"disk_type":           types.StringType,
		"product_description": types.StringType,
	}
}
func (m *mysqlProductModel) refreshFromOutput(output *vmysql.Product) {
	m.ProductCode = types.StringPointerValue(output.ProductCode)
	m.ProductName = types.StringPointerValue(output.ProductName)
	m.ProductType = types.StringPointerValue(output.ProductType.Code)
	m.ProductDescription = types.StringPointerValue(output.ProductDescription)
	m.InfraResourceType = types.StringPointerValue(output.InfraResourceType.Code)
	m.CpuCount = types.Int64Value(int64(*output.CpuCount))
	m.MemorySize = types.Int64PointerValue(output.MemorySize)
	m.DiskType = types.StringPointerValue(output.DiskType.Code)
}