/*
Copyright 2019 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
*/
package bigip

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	bigip "github.com/f5devcentral/go-bigip"
	"github.com/f5devcentral/go-bigip/f5teem"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var (
	mutex sync.Mutex
)

func resourceBigipAwafPolicy() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceBigipAwafPolicyCreate,
		ReadContext:   resourceBigipAwafPolicyRead,
		UpdateContext: resourceBigipAwafPolicyUpdate,
		DeleteContext: resourceBigipAwafPolicyDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The unique user-given name of the policy. Policy names cannot contain spaces or special characters. Allowed characters are a-z, A-Z, 0-9, dot, dash (-), colon (:) and underscore (_)",
				ForceNew:    true,
			},
			"partition": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "Common",
				ForceNew:     true,
				Description:  "Partition of WAF policy",
				ValidateFunc: validatePartitionName,
			},
			"template_name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Specifies the name of the template used for the policy creation.",
			},
			"template_link": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Specifies the Link of the template used for the policy creation.",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Specifies the description of the policy.",
			},
			"application_language": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "utf-8",
				Description: "The character encoding for the web application. The character encoding determines how the policy processes the character sets. The default is Auto detect",
			},
			"case_insensitive": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Specifies whether the security policy treats microservice URLs, file types, URLs, and parameters as case sensitive or not. When this setting is enabled, the system stores these security policy elements in lowercase in the security policy configuration",
			},
			"enable_passivemode": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Passive Mode allows the policy to be associated with a Performance L4 Virtual Server (using a FastL4 profile). With FastL4, traffic is analyzed but is not modified in any way.",
			},
			"protocol_independent": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "When creating a security policy, you can determine whether a security policy differentiates between HTTP and HTTPS URLs. If enabled, the security policy differentiates between HTTP and HTTPS URLs. If disabled, the security policy configures URLs without specifying a specific protocol. This is useful for applications that behave the same for HTTP and HTTPS, and it keeps the security policy from including the same URL twice.",
			},
			"enforcement_mode": {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  "How the system processes a request that triggers a security policy violation",
				ValidateFunc: validation.StringInSlice([]string{"blocking", "transparent"}, false),
			},
			"type": {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  "The type of policy you want to create. The default policy type is Security.",
				Default:      "security",
				ValidateFunc: validation.StringInSlice([]string{"parent", "security"}, false),
			},
			"server_technologies": {
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "The server technology is a server-side application, framework, web server or operating system type that is configured in the policy in order to adapt the policy to the checks needed for the respective technology.",
				Optional:    true,
			},
			"policy_id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"parameters": {
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional:    true,
				Description: "This section defines parameters that the security policy permits in requests.",
			},
			"urls": {
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional:    true,
				Description: "In a security policy, you can manually specify the HTTP URLs that are allowed (or disallowed) in traffic to the web application being protected. If you are using automatic policy building (and the policy includes learning URLs), the system can determine which URLs to add, based on legitimate traffic.",
			},
			"signature_sets": {
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional:    true,
				Description: "Defines behavior when signatures found within a signature-set are detected in a request. Settings are culmulative, so if a signature is found in any set with block enabled, that signature will have block enabled.",
			},
			"signatures": {
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional:    true,
				Description: "This section defines the properties of a signature on the policy.",
			},
			"signatures_settings": {
				Type:        schema.TypeSet,
				Description: "bulk signature setting",
				Optional:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"signature_staging": {
							Type:        schema.TypeBool,
							Optional:    true,
							Description: "setting true will enforce all signature from staging",
						},
						"placesignatures_in_staging": {
							Type:        schema.TypeBool,
							Optional:    true,
							Description: "",
						},
					},
				},
			},
			"policy_builder": {
				Type:        schema.TypeSet,
				Description: "policy-builder settings for policy",
				Optional:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"learning_mode": {
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validation.StringInSlice([]string{"automatic", "disabled", "manual"}, false),
						},
					},
				},
			},
			"graphql_profiles": {
				Type:        schema.TypeSet,
				Description: "graphql_profile settings for policy",
				Optional:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"host_names": {
				Type:        schema.TypeSet,
				Description: "specify the list of host name that is used to access the application",
				Optional:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"file_types": {
				Type:        schema.TypeSet,
				Description: "file_types settings for policy",
				Optional:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"type": {
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validation.StringInSlice([]string{"explicit", "wildcard"}, false),
						},
					},
				},
			},
			"open_api_files": {
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional:    true,
				Description: "This section defines the Link for open api files on the policy.",
			},
			"modifications": {
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional:    true,
				Description: " the modifications section includes actions that modify the declarative policy as it is defined in the adjustments section. The modifications section is updated manually, with the changes generally driven by the learning suggestions provided by the BIG-IP.",
			},
			"policy_import_json": {
				Type:     schema.TypeString,
				Optional: true,
				//Computed:    true,
				Description: "The payload of the WAF Policy to be used for IMPORT on to BIGIP",
			},
			"policy_export_json": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The payload of the WAF Policy to be EXPORTED from BIGIP to OUTPUT",
			},
		},
	}
}

func resourceBigipAwafPolicyCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	name := d.Get("name").(string)
	partition := d.Get("partition").(string)

	log.Println("[INFO] AWAF Policy Name " + name)

	provision := "asm"
	p, err := client.Provisions(provision)
	if err != nil {
		log.Printf("[ERROR] Unable to Retrieve Provision (%s) (%v) ", provision, err)
		return diag.FromErr(err)
	}
	if p.Level == "none" {
		return diag.FromErr(fmt.Errorf("[ERROR] ASM Module is not provisioned, it is set to : (%s) ", p.Level))
	}

	config, err := getpolicyConfig(d)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error in Json encode for waf policy %+v", err))
	}
	polName := fmt.Sprintf("/%s/%s", partition, name)
	mutex.Lock()
	log.Printf("[INFO] AWAF Policy Config: %+v ", config)
	// os.WriteFile("awaf_output.json", []byte(config), 0644)
	taskId, err := client.ImportAwafJson(polName, config, "")
	log.Printf("[INFO] AWAF Import policy TaskID :%v", taskId)
	if err != nil {
		return diag.FromErr(fmt.Errorf("Error in Importing AWAF json (%s): %s ", name, err))
	}
	err = client.GetImportStatus(taskId)
	if err != nil {
		return diag.FromErr(fmt.Errorf("Error in Importing AWAF json (%s): %s ", name, err))
	}
	part := strings.Split(partition, "/")[0]
	time.Sleep(10 * time.Second)
	wafpolicy, err := client.GetWafPolicyQuery(name, part)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error retrieving waf policy %+v: %v", wafpolicy, err))
	}
	taskId, err = client.ApplyAwafJson(polName, wafpolicy.ID)
	log.Printf("[INFO] AWAF Apply policy TaskID :%v", taskId)
	if err != nil {
		err1 := client.DeleteWafPolicy(wafpolicy.ID)
		if err1 != nil {
			return diag.FromErr(fmt.Errorf(" Error Deleting AWAF Policy : %s", err1))
		}
		return diag.FromErr(fmt.Errorf("Error in Applying AWAF json (%s): %s ", name, err))
	}
	err = client.GetApplyStatus(taskId)
	if err != nil {
		return diag.FromErr(fmt.Errorf("Error in Applying AWAF json (%s): %s ", name, err))
	}

	if !client.Teem {
		id := uuid.New()
		uniqueID := id.String()
		assetInfo := f5teem.AssetInfo{
			Name:    "Terraform-provider-bigip",
			Version: client.UserAgent,
			Id:      uniqueID,
		}
		apiKey := os.Getenv("TEEM_API_KEY")
		teemDevice := f5teem.AnonymousClient(assetInfo, apiKey)
		f := map[string]interface{}{
			"waf_policy_name":            name,
			"waf_policy_id":              wafpolicy.ID,
			"Number_of_entity_url":       len(d.Get("urls").([]interface{})),
			"Number_of_entity_parameter": len(d.Get("parameters").([]interface{})),
			"Terraform Version":          client.UserAgent,
		}
		tsVer := strings.Split(client.UserAgent, "/")
		err = teemDevice.Report(f, "bigip_waf_policy", tsVer[3])
		if err != nil {
			log.Printf("[ERROR]Sending Telemetry data failed:%v", err)
		}
	}
	d.SetId(wafpolicy.ID)
	mutex.Unlock()
	return resourceBigipAwafPolicyRead(ctx, d, meta)
}

func resourceBigipAwafPolicyRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	policyID := d.Id()
	name := d.Get("name").(string)

	log.Printf("[INFO] Reading AWAF Policy %v with ID: %+v", name, policyID)

	wafpolicy, err := client.GetWafPolicy(policyID)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error retrieving waf policy %+v: %v", wafpolicy, err))
	}

	policyJson, err := client.ExportPolicy(policyID)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error Exporting waf policy `%+v` with : %v", name, err))
	}
	// plJson, err := json.Marshal(policyJson.Policy)
	plJson, err := client.ExportPolicyFull(policyID)
	if err != nil {
		return diag.FromErr(err)
	}
	_ = d.Set("name", policyJson.Policy.Name)
	part := strings.Split(policyJson.Policy.FullPath, "/")
	_ = d.Set("partition", part[1])
	if len(part) > 3 {
		_ = d.Set("partition", fmt.Sprintf("%s/%s", part[1], part[2]))
	}
	_ = d.Set("policy_id", wafpolicy.ID)
	_ = d.Set("type", policyJson.Policy.Type)
	_ = d.Set("application_language", policyJson.Policy.ApplicationLanguage)
	if _, ok := d.GetOk("enforcement_mode"); ok {
		_ = d.Set("enforcement_mode", policyJson.Policy.EnforcementMode)
	}
	if _, ok := d.GetOk("description"); ok {
		_ = d.Set("description", policyJson.Policy.Description)
	}
	_ = d.Set("template_name", policyJson.Policy.Template.Name)
	// _ = d.Set("template_link", policyJson.Policy.Template.Link)
	_ = d.Set("policy_export_json", plJson)

	return nil
}

func resourceBigipAwafPolicyUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	policyID := d.Id()
	name := d.Get("name").(string)
	partition := d.Get("partition").(string)
	log.Printf("[INFO] Updating AWAF Policy : %+v", name)

	config, err := getpolicyConfig(d)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error in Json encode for waf policy %+v", err))
	}
	log.Printf("[DEBUG] Policy config: %+v", config)
	polName := fmt.Sprintf("/%s/%s", partition, name)
	mutex.Lock()
	taskId, err := client.ImportAwafJson(polName, config, policyID)
	log.Printf("[DEBUG] AWAF Import policy TaskID :%v", taskId)
	if err != nil {
		return diag.FromErr(fmt.Errorf("Error in Importing AWAF json (%s): %s ", name, err))
	}
	err = client.GetImportStatus(taskId)
	if err != nil {
		return diag.FromErr(fmt.Errorf("Error in Importing AWAF json (%s): %s ", name, err))
	}
	taskId, err = client.ApplyAwafJson(polName, policyID)
	log.Printf("[INFO] AWAF Apply policy TaskID :%v", taskId)
	if err != nil {
		return diag.FromErr(fmt.Errorf("Error in Applying AWAF json (%s): %s ", name, err))
	}
	err = client.GetApplyStatus(taskId)
	if err != nil {
		return diag.FromErr(fmt.Errorf("Error in Applying AWAF json (%s): %s ", name, err))
	}
	mutex.Unlock()
	return resourceBigipAwafPolicyRead(ctx, d, meta)
}

func resourceBigipAwafPolicyDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	policyID := d.Id()
	name := d.Get("name").(string)
	log.Printf("[INFO] Deleting AWAF Policy : %+v with ID: %+v", name, policyID)

	err := client.DeleteWafPolicy(policyID)
	if err != nil {
		return diag.FromErr(fmt.Errorf(" Error Deleting AWAF Policy : %s", err))
	}
	d.SetId("")
	return nil
}

func getpolicyConfig(d *schema.ResourceData) (string, error) {
	name := d.Get("name").(string)
	partition := d.Get("partition").(string)
	var fullPath string
	if partition != "Common" {
		fullPath = fmt.Sprintf("/%s/%s", partition, name)
	}
	policyWaf := bigip.WafPolicy{
		Name:                name,
		Partition:           partition,
		FullPath:            fullPath,
		ApplicationLanguage: d.Get("application_language").(string),
	}
	policyWaf.CaseInsensitive = d.Get("case_insensitive").(bool)
	policyWaf.EnablePassiveMode = d.Get("enable_passivemode").(bool)
	policyWaf.ProtocolIndependent = d.Get("protocol_independent").(bool)
	policyWaf.EnforcementMode = d.Get("enforcement_mode").(string)
	policyWaf.Description = d.Get("description").(string)
	if val, ok := d.GetOk("signatures_settings"); ok {
		for _, item := range val.(*schema.Set).List() {
			log.Printf("Value:%+v", item.(map[string]interface{})["signature_staging"].(bool))
			policyWaf.SignatureSettings.SignatureStaging = item.(map[string]interface{})["signature_staging"].(bool)
		}
	}
	if val, ok := d.GetOk("policy_builder"); ok {
		for _, item := range val.(*schema.Set).List() {
			log.Printf("Value:%+v", item.(map[string]interface{})["learning_mode"].(string))
			policyWaf.PolicyBuilder.LearningMode = item.(map[string]interface{})["learning_mode"].(string)
		}
	}
	var graphProfles []bigip.GraphqlProfile
	if val, ok := d.GetOk("graphql_profiles"); ok {
		var gralPro bigip.GraphqlProfile
		for _, item := range val.(*schema.Set).List() {
			gralPro.Name = item.(map[string]interface{})["name"].(string)
			graphProfles = append(graphProfles, gralPro)
		}
	}
	policyWaf.GraphqlProfiles = graphProfles

	var hostNames []bigip.HostName
	if val, ok := d.GetOk("host_names"); ok {
		var hostName bigip.HostName
		for _, item := range val.(*schema.Set).List() {
			hostName.Name = item.(map[string]interface{})["name"].(string)
			hostName.IncludeSubdomains = true
			hostNames = append(hostNames, hostName)
		}
	}
	policyWaf.HostNames = hostNames
	var fileTypes []bigip.Filetype
	if val, ok := d.GetOk("file_types"); ok {
		var fileType bigip.Filetype
		for _, item := range val.(*schema.Set).List() {
			fileType.Name = item.(map[string]interface{})["name"].(string)
			fileType.Type = item.(map[string]interface{})["type"].(string)
			fileTypes = append(fileTypes, fileType)
		}
	}
	policyWaf.Filetypes = fileTypes
	policyWaf.Type = d.Get("type").(string)
	policyWaf.Template = struct {
		Name string `json:"name,omitempty"`
		Link string `json:"link,omitempty"`
	}{
		Name: d.Get("template_name").(string),
	}
	if _, ok := d.GetOk("template_link"); ok {
		policyWaf.Template = struct {
			Name string `json:"name,omitempty"`
			Link string `json:"link,omitempty"`
		}{
			Link: d.Get("template_link").(string),
		}
	}
	p := d.Get("server_technologies").([]interface{})

	var sts []bigip.ServerTech

	for i := 0; i < len(p); i++ {
		var stec bigip.ServerTech
		stec.ServerTechnologyName = p[i].(string)
		sts = append(sts, stec)
	}
	policyWaf.ServerTechnologies = sts

	var wafUrls []bigip.WafUrlJson
	urls := d.Get("urls").([]interface{})
	for i := 0; i < len(urls); i++ {
		var wafUrl bigip.WafUrlJson
		_ = json.Unmarshal([]byte(urls[i].(string)), &wafUrl)
		wafUrls = append(wafUrls, wafUrl)
	}
	policyWaf.Urls = wafUrls

	var wafParams []bigip.Parameter
	parmtrs := d.Get("parameters").([]interface{})
	for i := 0; i < len(parmtrs); i++ {
		var wafParam bigip.Parameter
		_ = json.Unmarshal([]byte(parmtrs[i].(string)), &wafParam)
		wafParams = append(wafParams, wafParam)
	}
	policyWaf.Parameters = wafParams

	var wafsigSets []bigip.SignatureSet
	sigSets := d.Get("signature_sets").([]interface{})
	for i := 0; i < len(sigSets); i++ {
		var sigSet bigip.SignatureSet
		_ = json.Unmarshal([]byte(sigSets[i].(string)), &sigSet)
		wafsigSets = append(wafsigSets, sigSet)
	}
	policyWaf.SignatureSets = wafsigSets

	var wafsigSigns []bigip.WafSignature
	sigNats := d.Get("signatures").([]interface{})
	for i := 0; i < len(sigNats); i++ {
		var sigNat bigip.WafSignature
		_ = json.Unmarshal([]byte(sigNats[i].(string)), &sigNat)
		wafsigSigns = append(wafsigSigns, sigNat)
	}
	policyWaf.Signatures = wafsigSigns

	var openApiLinks []bigip.OpenApiLink
	apiLinks := d.Get("open_api_files").([]interface{})
	for i := 0; i < len(apiLinks); i++ {
		var apiLink bigip.OpenApiLink
		apiLink.Link = apiLinks[i].(string)
		openApiLinks = append(openApiLinks, apiLink)
	}
	policyWaf.OpenAPIFiles = openApiLinks

	// policyJson := &bigip.PolicyStruct{}
	policyJson := &bigip.PolicyStructobject{}
	policyJson.Policy = policyWaf

	if val, ok := d.GetOk("policy_import_json"); ok {
		var polJsn bigip.PolicyStruct
		_ = json.Unmarshal([]byte(val.(string)), &polJsn)
		var polJsn1 bigip.PolicyStructobject
		_ = json.Unmarshal([]byte(val.(string)), &polJsn1)
		if polJsn1.Policy.(map[string]interface{})["fullPath"] != policyWaf.Name {
			polJsn1.Policy.(map[string]interface{})["fullPath"] = fmt.Sprintf("/%s/%s", policyWaf.Partition, policyWaf.Name)
			polJsn1.Policy.(map[string]interface{})["name"] = policyWaf.Name
		}
		if policyWaf.Template.Name != "" && polJsn1.Policy.(map[string]interface{})["template"] != policyWaf.Template {
			polJsn1.Policy.(map[string]interface{})["template"] = policyWaf.Template
		}
		if policyWaf.ApplicationLanguage != "" {
			polJsn1.Policy.(map[string]interface{})["applicationLanguage"] = policyWaf.ApplicationLanguage
		}
		urlList := make([]interface{}, len(policyWaf.Urls))
		for i, v := range policyWaf.Urls {
			urlList[i] = v
		}
		_, urlsOK := polJsn1.Policy.(map[string]interface{})["urls"]
		if urlsOK {
			urlLL := append(polJsn1.Policy.(map[string]interface{})["urls"].([]interface{}), urlList...)
			polJsn1.Policy.(map[string]interface{})["urls"] = urlLL
		} else {
			polJsn1.Policy.(map[string]interface{})["urls"] = urlList
		}

		params := make([]interface{}, len(policyWaf.Parameters))
		for i, v := range policyWaf.Parameters {
			params[i] = v
		}
		_, paramsOK := polJsn1.Policy.(map[string]interface{})["parameters"]
		if paramsOK {
			paramsLL := append(polJsn1.Policy.(map[string]interface{})["parameters"].([]interface{}), params...)
			polJsn1.Policy.(map[string]interface{})["parameters"] = paramsLL
		} else {
			polJsn1.Policy.(map[string]interface{})["parameters"] = params
		}

		sigSet := make([]interface{}, len(policyWaf.SignatureSets))
		for i, v := range policyWaf.SignatureSets {
			sigSet[i] = v
		}
		_, sigSetOK := polJsn1.Policy.(map[string]interface{})["signature-sets"]
		if sigSetOK {
			sigSetsList := append(polJsn1.Policy.(map[string]interface{})["signature-sets"].([]interface{}), sigSet...)
			polJsn1.Policy.(map[string]interface{})["signature-sets"] = sigSetsList
		} else {
			polJsn1.Policy.(map[string]interface{})["signature-sets"] = sigSet
		}

		fileType := make([]interface{}, len(policyWaf.Filetypes))
		for i, v := range policyWaf.Filetypes {
			fileType[i] = v
		}
		_, fileTyOK := polJsn1.Policy.(map[string]interface{})["filetypes"]
		if fileTyOK {
			fileTypeList := append(polJsn1.Policy.(map[string]interface{})["filetypes"].([]interface{}), fileType...)
			polJsn1.Policy.(map[string]interface{})["filetypes"] = fileTypeList
		} else {
			polJsn1.Policy.(map[string]interface{})["filetypes"] = fileType
		}
		hostName := make([]interface{}, len(policyWaf.HostNames))
		for i, v := range policyWaf.HostNames {
			hostName[i] = v
		}
		_, hostTyOK := polJsn1.Policy.(map[string]interface{})["host-names"]
		if hostTyOK {
			hostNameList := append(polJsn1.Policy.(map[string]interface{})["host-names"].([]interface{}), hostName...)
			polJsn1.Policy.(map[string]interface{})["host-names"] = hostNameList
		} else {
			polJsn1.Policy.(map[string]interface{})["host-names"] = hostName
		}
		if policyWaf.Description != "" {
			polJsn1.Policy.(map[string]interface{})["description"] = policyWaf.Description
		}
		serverTech := make([]interface{}, len(policyWaf.ServerTechnologies))
		for i, v := range policyWaf.ServerTechnologies {
			serverTech[i] = v
		}
		_, srvrTCOK := polJsn1.Policy.(map[string]interface{})["server-technologies"]
		if srvrTCOK {
			serverTechList := append(polJsn1.Policy.(map[string]interface{})["server-technologies"].([]interface{}), serverTech...)
			polJsn1.Policy.(map[string]interface{})["server-technologies"] = serverTechList
		} else {
			polJsn1.Policy.(map[string]interface{})["server-technologies"] = serverTech
		}
		openApi := make([]interface{}, len(policyWaf.OpenAPIFiles))
		for i, v := range policyWaf.OpenAPIFiles {
			openApi[i] = v
		}
		_, openApiOK := polJsn1.Policy.(map[string]interface{})["open-api-files"]
		if openApiOK {
			openApiList := append(polJsn1.Policy.(map[string]interface{})["open-api-files"].([]interface{}), openApi...)
			polJsn1.Policy.(map[string]interface{})["open-api-files"] = openApiList
		} else {
			polJsn1.Policy.(map[string]interface{})["open-api-files"] = openApi
		}

		graphQL := make([]interface{}, len(policyWaf.GraphqlProfiles))
		for i, v := range policyWaf.GraphqlProfiles {
			graphQL[i] = v
		}
		_, graphqlOK := polJsn1.Policy.(map[string]interface{})["graphql-profiles"]
		if graphqlOK {
			graphQLL := append(polJsn1.Policy.(map[string]interface{})["graphql-profiles"].([]interface{}), graphQL...)
			polJsn1.Policy.(map[string]interface{})["graphql-profiles"] = graphQLL
		} else {
			polJsn1.Policy.(map[string]interface{})["graphql-profiles"] = graphQL
		}

		var myModification []interface{}
		if val, ok := d.GetOk("modifications"); ok {
			if x, ok := val.([]interface{}); ok {
				for _, e := range x {
					pb := []byte(e.(string))
					var tmp interface{}
					_ = json.Unmarshal(pb, &tmp)
					myMap := tmp.(map[string]interface{})
					pbList := myMap["suggestions"]
					myModification = append(myModification, pbList.([]interface{})...)
				}
			}
		}
		polJsn1.Modifications = myModification
		log.Printf("[DEBUG] Modifications: %+v", polJsn1.Modifications)
		// log.Printf("[INFO][Import] Policy Json: %+v", polJsn1)
		data, err := json.Marshal(polJsn1)
		if err != nil {
			return "", err
		}
		// _ = os.WriteFile("myimport.json", data, 0644)
		return string(data), nil

		// if polJsn.Policy.FullPath != policyWaf.Name {
		//	polJsn.Policy.FullPath = fmt.Sprintf("/%s/%s", policyWaf.Partition, policyWaf.Name)
		//	polJsn.Policy.Name = policyWaf.Name
		// }
		// if policyWaf.Template.Name != "" && polJsn.Policy.Template != policyWaf.Template {
		//	polJsn.Policy.Template = policyWaf.Template
		// }
		// polJsn.Policy.Urls = append(polJsn.Policy.Urls, policyWaf.Urls...)
		// polJsn.Policy.Parameters = []bigip.Parameter{}
		// if policyWaf.Parameters != nil && len(policyWaf.Parameters) > 0 && policyWaf.Parameters[0].Name != "*" {
		//  	polJsn.Policy.Parameters = append(polJsn.Policy.Parameters, policyWaf.Parameters...)
		// }
		// polJsn.Policy.GraphqlProfiles = append(polJsn.Policy.GraphqlProfiles, policyWaf.GraphqlProfiles...)
		// policyJson.Policy = polJsn.Policy
	}

	var myModification []interface{}
	if val, ok := d.GetOk("modifications"); ok {
		if x, ok := val.([]interface{}); ok {
			for _, e := range x {
				pb := []byte(e.(string))
				var tmp interface{}
				_ = json.Unmarshal(pb, &tmp)
				myMap := tmp.(map[string]interface{})
				pbList := myMap["suggestions"]
				myModification = append(myModification, pbList.([]interface{})...)
			}
		}
	}
	policyJson.Modifications = myModification
	log.Printf("[DEBUG] Modifications: %+v", policyJson.Modifications)
	log.Printf("[DEBUG] Policy Json: %+v", policyJson)
	data, err := json.Marshal(policyJson)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
