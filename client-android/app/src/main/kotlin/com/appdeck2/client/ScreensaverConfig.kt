package com.appdeck2.client

data class ScreensaverConfig(
    val enabled: Boolean,
    val timeoutMinutes: Int,
    val defaultVideoId: String,
    val defaultVideoHash: String,
    val tenantVideoId: String,
    val tenantVideoHash: String,
    val tenantStartDate: String,
    val tenantEndDate: String,
    val tenantStartTime: String,
    val tenantEndTime: String
)
