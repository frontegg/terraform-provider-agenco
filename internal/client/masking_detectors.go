package client

import "sort"

// maskingDetectors maps the Terraform detector name to the field the masking policy API
// expects. Add new entries when the API gains detectors.
var maskingDetectors = map[string]string{
	"aba_routing_number":                           "abaRoutingNumber",
	"au_abn":                                       "auAbn",
	"au_acn":                                       "auAcn",
	"au_medicare":                                  "auMedicare",
	"au_tfn":                                       "auTfn",
	"australia_passport_number":                    "australiaPassportNumber",
	"austria_ssn":                                  "austriaSsn",
	"bitcoin_address":                              "bitcoinAddress",
	"br_cpf_number":                                "brCpfNumber",
	"brazil_cnpj_number":                           "brazilCnpjNumber",
	"bulgarian_uniform_civil_number":               "bulgarianUniformCivilNumber",
	"canada_bank_account_number":                   "canadaBankAccountNumber",
	"canada_drivers_license_number":                "canadaDriversLicenseNumber",
	"canada_health_service_number":                 "canadaHealthServiceNumber",
	"canada_passport":                              "canadaPassport",
	"canada_personal_health_id_number_phin":        "canadaPersonalHealthIdNumberPhin",
	"canada_social_insurance_number":               "canadaSocialInsuranceNumber",
	"china_id_number":                              "chinaIdNumber",
	"credit_card":                                  "creditCard",
	"croatia_id_number":                            "croatiaIdNumber",
	"cvv_cvc":                                      "cvvCvc",
	"dutch_bank_account_number":                    "dutchBankAccountNumber",
	"email_address":                                "emailAddress",
	"es_nif":                                       "esNif",
	"ethereum_address":                             "ethereumAddress",
	"france_ssn_nir":                               "franceSsnNir",
	"germany_drivers_license_number":               "germanyDriversLicenseNumber",
	"germany_id_number":                            "germanyIdNumber",
	"germany_passport_number":                      "germanyPassportNumber",
	"germany_tax_id_number":                        "germanyTaxIdNumber",
	"germany_vat_number":                           "germanyVatNumber",
	"hong_kong_id":                                 "hongKongId",
	"iban_code":                                    "ibanCode",
	"il_bank_number":                               "ilBankNumber",
	"il_id_number":                                 "ilIdNumber",
	"il_passport_re":                               "ilPassportRe",
	"ip_address":                                   "ipAddress",
	"it_driver_license":                            "itDriverLicense",
	"it_fiscal_code":                               "itFiscalCode",
	"it_identity_card":                             "itIdentityCard",
	"it_passport":                                  "itPassport",
	"it_vat_code":                                  "itVatCode",
	"japan_bank_account_number":                    "japanBankAccountNumber",
	"japan_driver_license_number":                  "japanDriverLicenseNumber",
	"japan_my_number_corporate":                    "japanMyNumberCorporate",
	"japan_my_number_personal":                     "japanMyNumberPersonal",
	"japan_passport_number":                        "japanPassportNumber",
	"japan_residence_card_number":                  "japanResidenceCardNumber",
	"japan_resident_registration_number":           "japanResidentRegistrationNumber",
	"japan_social_insurance_number_sin":            "japanSocialInsuranceNumberSin",
	"malaysia_id_number":                           "malaysiaIdNumber",
	"medical_license":                              "medicalLicense",
	"mexico_unique_population_registry_code_curp":  "mexicoUniquePopulationRegistryCodeCurp",
	"netherland_bsn":                               "netherlandBsn",
	"new_zealand_nhi_number":                       "newZealandNhiNumber",
	"phone_number":                                 "phoneNumber",
	"public_ip_address":                            "publicIpAddress",
	"sg_nric_fin":                                  "sgNricFin",
	"singapore_driver_license_number":              "singaporeDriverLicenseNumber",
	"singapore_passport_number":                    "singaporePassportNumber",
	"south_korea_alien_registeration_number":       "southKoreaAlienRegisterationNumber",
	"south_korea_credit_card_number":               "southKoreaCreditCardNumber",
	"south_korea_domestic_residence_report_number": "southKoreaDomesticResidenceReportNumber",
	"south_korea_driver_license_number":            "southKoreaDriverLicenseNumber",
	"south_korea_health_insurance_number":          "southKoreaHealthInsuranceNumber",
	"south_korea_id_number":                        "southKoreaIdNumber",
	"south_korea_passport_mrz_number":              "southKoreaPassportMrzNumber",
	"south_korea_passport_number":                  "southKoreaPassportNumber",
	"south_korea_resident_registration_number":     "southKoreaResidentRegistrationNumber",
	"spain_id_dni":                                 "spainIdDni",
	"spain_ssn":                                    "spainSsn",
	"swift_code":                                   "swiftCode",
	"switzerland_ssn":                              "switzerlandSsn",
	"taiwan_national_id_number":                    "taiwanNationalIdNumber",
	"thailand_id_number":                           "thailandIdNumber",
	"uae_identity_card_number":                     "uaeIdentityCardNumber",
	"uae_passport":                                 "uaePassport",
	"uk_driver_license_number":                     "ukDriverLicenseNumber",
	"uk_electoral_roll_number":                     "ukElectoralRollNumber",
	"uk_national_insurance_number_nino":            "ukNationalInsuranceNumberNino",
	"uk_nhs":                                       "ukNhs",
	"uk_unique_taxpayer_reference_number":          "ukUniqueTaxpayerReferenceNumber",
	"unverified_credit_card":                       "unverifiedCreditCard",
	"url":                                          "url",
	"us_bank_number":                               "usBankNumber",
	"us_driver_license":                            "usDriverLicense",
	"us_itin":                                      "usItin",
	"us_passport":                                  "usPassport",
	"us_ssn":                                       "usSsn",
	"vin":                                          "vin",
}

// MaskingDetectorNames returns every supported detector name in sorted order.
func MaskingDetectorNames() []string {
	names := make([]string, 0, len(maskingDetectors))
	for name := range maskingDetectors {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// MaskingDetectorAPIField resolves a Terraform detector name to its API field name.
func MaskingDetectorAPIField(name string) (string, bool) {
	field, ok := maskingDetectors[name]
	return field, ok
}

// MaskingDetectorFromAPIField resolves an API field name back to its Terraform name.
func MaskingDetectorFromAPIField(field string) (string, bool) {
	for name, apiField := range maskingDetectors {
		if apiField == field {
			return name, true
		}
	}
	return "", false
}
