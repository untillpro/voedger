-- Copyright (c) 2020-present unTill Pro, Ltd.
-- @author Denis Gribanov

APPLICATION app1();

ALTERABLE WORKSPACE test_ws (
	TABLE articles INHERITS CDoc (
		name varchar,
		article_manual int32 NOT NULL,
		article_hash int32 NOT NULL,
		hideonhold int32 NOT NULL,
		time_active int32 NOT NULL,
		control_active int32 NOT NULL
	);

	TABLE options INHERITS CDoc ();

	TABLE department INHERITS CDoc (
		pc_fix_button int32 NOT NULL,
		rm_fix_button int32 NOT NULL,
		id_food_group ref,
		department_options TABLE department_options (
			id_department ref NOT NULL,
			id_options ref,
			option_number int32,
			option_type int32
		)
	);

	TABLE air_table_plan INHERITS CDoc (
		fstate int32,
		name varchar,
		ml_name bytes,
		num int32,
		width int32,
		height int32,
		image int64,
		is_hidden int32,
		preview int64,
		bg_color int32,
		air_table_plan_item TABLE air_table_plan_item (
			id_air_table_plan ref, --deprecated link to air_table_plan
			fstate int32,
			number int32,
			form int32 NOT NULL,
			top_c int32,
			left_c int32,
			angle int32,
			width int32,
			height int32,
			places int32,
			chair_type varchar,
			table_type varchar,
			type int32,
			color int32,
			code varchar,
			text varchar,
			hide_seats bool
		)
	);

	TABLE printers INHERITS CDoc (
		guid varchar NOT NULL,
		name varchar,
		id_printer_drivers ref,
		width int32,
		top_lines int32,
		bottom_lines int32,
		con int32,
		port int32,
		speed int32,
		backup_printer varchar,
		id_computers ref,
		error_flag int32 NOT NULL,
		codepage int32,
		null_print int32,
		fiscal int32,
		dont_auto_open_drawer int32,
		connection_type int32,
		printer_ip varchar,
		printer_port int32,
		cant_be_redirected_to int32,
		com_params bytes,
		printer_type int32,
		exclude_message int32,
		driver_kind int32,
		driver_id varchar,
		driver_params bytes,
		check_status int32,
		id_ordermans ref,
		id_falcon_terminals ref,
		hht_printer_port  int32,
		ml_name bytes,
		posprinter_driver_id varchar,
		posprinter_driver_params varchar,
		id_bill_ticket ref,
		id_order_ticket ref,
		purpose_receipt_enabled bool,
		purpose_preparation_enabled bool
	);

	TABLE sales_area INHERITS CDoc (
		name varchar,
		bmanual int32 NOT NULL,
		id_prices ref,
		number int32,
		close_manualy int32,
		auto_accept_reservations int32,
		only_reserved int32,
		id_prices_original int64,
		group_vat_level int32,
		sc int64,
		sccovers int32,
		id_scplan ref,
		price_dt int64,
		sa_external_id varchar,
		is_default bool,
		id_table_plan ref
	);

	TABLE payments INHERITS CDoc (
		name varchar,
		kind int32,
		number int32,
		psp_model int32,
		id_bookkp ref,
		id_currency ref,
		params varchar,
		driver_kind int32,
		driver_id varchar,
		guid varchar,
		ml_name bytes,
		paym_external_id varchar
	);

	TABLE untill_users INHERITS CDoc (
		name varchar,
		mandates bytes,
		user_void int32 NOT NULL,
		user_code varchar,
		user_card varchar,
		language varchar,
		language_char int32,
		user_training int32,
		address varchar,
		id_countries ref,
		phone varchar,
		datebirth int64,
		insurance varchar,
		user_poscode varchar,
		terminal_id varchar,
		user_clock_in int32,
		user_poscode_remoteterm varchar,
		is_custom_remoteterm_poscode int32,
		id_group_users ref,
		tp_api_pwd varchar,
		firstname varchar,
		lastname varchar,
		user_transfer int32,
		personal_drawer int32,
		start_week_day int32,
		start_week_time int32,
		needcashdeclaration int32,
		smartcard_uid varchar,
		not_print_waiter_report int32,
		exclude_message int32,
		lefthand int32,
		login_message varchar,
		email varchar,
		number int32,
		hq_id varchar,
		void_number int32,
		last_update_dt int64,
		block_time_break int32,
		void_type int32,
		tpapi_permissions bytes,
		hide_wm int32,
		creation_date int64
	);

	TABLE computers INHERITS CDoc (
		name varchar,
		show_cursor int32,
		on_hold int32,
		untillsrv_port int32,
		id_screen_groups ref,
		id_tickets_clock ref,
		guid_printers_clock varchar,
		keyboard_input_text int32,
		extra_data varchar,
		extra_data_new varchar,
		startup_message varchar,
		guid_cash_printers varchar,
		id_cash_tickets ref,
		term_uid int32,
		production_nr varchar,
		tpapi int32,
		ignore_prn_errors bytes,
		default_a4_printer varchar,
		login_screen int32,
		id_themes ref,
		device_profile_wsid int64,
		restaurant_computers TABLE restaurant_computers (
			id_computers ref NOT NULL,
			id_sales_area ref,
			sales_kind int32,
			id_printers_1 ref,
			keep_waiter int32,
			limited int32,
			id_periods ref,
			dbl int32,
			a4 int32,
			id_screens_part ref,
			id_screens_order ref,
			id_screens_supplement ref,
			id_screens_condiment ref,
			id_screens_payment ref,
			id_tickets_bill ref,
			id_printers_proforma ref,
			id_tickets_proforma ref,
			direct_table int32,
			start_table int32,
			id_psp_layout ref,
			id_deposit_layout ref,
			id_deposit_printer ref,
			id_invoice_layout ref,
			id_rear_disp_printer ref,
			id_rear_disp_article_layout ref,
			id_rear_disp_bill_layout ref,
			id_journal_printer ref,
			auto_logoff_sec int32,
			id_tickets_journal ref,
			future_table int32,
			id_beco_location ref,
			id_tickets_order_journal ref,
			id_tickets_control_journal ref,
			id_drawer_layout ref,
			table_info varchar,
			table_pc_font bytes,
			table_hht_font bytes,
			id_return_layout ref,
			id_inout_layout ref,
			id_inout_printer ref,
			id_rent_layout ref,
			id_rent_printer ref,
			id_tickets_preauth ref,
			id_oif_preparation_area ref,
			id_reprint_order ref,
			id_rear_screen_saver ref,
			screen_saver_min int32,
			notprintlogoff int32,
			notprintnoorder int32,
			block_new_client int32,
			id_init_ks ref,
			id_tickets_giftcards ref,
			id_printers_giftcards ref,
			t2o_prepaid_tablenr int32,
			t2o_groups_table_from int32,
			t2o_groups_table_till int32,
			t2o_clients_table_from int32,
			t2o_clients_table_till int32,
			ao_order_direct_sales int32,
			ao_order_to_table int32,
			ao_table_nr int32,
			not_logoff_hht int32,
			id_printers_voucher ref,
			id_tickets_voucher ref,
			id_email_invoice_layout ref,
			id_printers_manager ref,
			id_tickets_manager ref,
			id_stock_location ref,
			on_hold_printing int32,
			id_ticket_voucher_bunch ref,
			id_ticket_voucher_bill ref,
			id_stock_printer ref,
			id_coupon_layout ref,
			id_printers_taorder ref,
			id_tickets_taorder ref,
			id_second_article_layout ref,
			second_article_delay_sec int32,
			id_printers_void ref,
			id_tickets_void ref,
			id_tickets_fiscal_footer ref,
			temp_orders_table_from int32,
			temp_orders_table_to int32,
			id_init_ksc ref,
			use_word_template_print_invoice int32,
			id_ta_total_layout ref,
			notify_blocked_card int32,
			id_printers_reopen ref,
			id_tickets_reopen ref,
			notify_blocked_card_layer int32,
			id_tickets_prof_fiscal_footer ref,
			id_tickets_giftcardsbill ref,
			id_printers_giftcardsbill ref
		)
	);

	TABLE bill INHERITS CDoc (
		tableno int32 NOT NULL,
		id_untill_users ref NOT NULL,
		table_part varchar NOT NULL,
		id_courses ref,
		id_clients ref,
		name varchar,
		proforma int32 NOT NULL,
		modified int64,
		open_datetime int64,
		close_datetime int64,
		number int32,
		failurednumber int32,
		suffix varchar,
		pbill_number int32,
		pbill_failurednumber int32,
		pbill_suffix varchar,
		hc_foliosequence int32,
		hc_folionumber varchar,
		tip int64,
		qty_persons int32,
		isdirty int32,
		reservationid varchar,
		id_alter_user ref,
		service_charge float64,
		number_of_covers int32,
		id_user_proforma ref,
		bill_type int32,
		locker int32,
		id_time_article ref,
		timer_start int64,
		timer_stop int64,
		isactive int32,
		table_name varchar,
		group_vat_level int32,
		comments varchar,
		id_cardprice ref,
		discount float64,
		discount_value int64,
		id_discount_reasons ref,
		hc_roomnumber varchar,
		ignore_auto_sc int32,
		extra_fields bytes,
		id_bo_service_charge ref,
		free_comments varchar,
		id_t2o_groups ref,
		service_tax int64,
		sc_plan bytes,
		client_phone varchar,
		age int64,
		description bytes,
		sdescription varchar,
		vars bytes,
		take_away int32,
		fiscal_number int32,
		fiscal_failurednumber int32,
		fiscal_suffix varchar,
		id_order_type ref,
		not_paid int64,
		total int64,
		was_cancelled int32,
		id_callers_last ref,
		id_serving_time ref,
		serving_time_dt int64,
		vat_excluded int32,
		day_number int32,
		day_failurednumber int32,
		day_suffix varchar,
		ayce_time int64,
		remaining_quantity float64,
		working_day varchar
	);

	TABLE pos_emails INHERITS CDoc (
		kind int32,
		email varchar,
		description varchar
	);

	TABLE WSKind INHERITS Singleton (
		IntFld int32 NOT NULL,
		StrFld varchar
	);

	TABLE category INHERITS CDoc (
		name varchar,
		hq_id varchar,
		ml_name bytes,
		cat_external_id varchar
	);

	TABLE Doc INHERITS CDoc (
		EmailField varchar NOT NULL VERIFIABLE,
		PhoneField varchar,
		NonVerifiedField varchar
	);

	TABLE DocConstraints INHERITS CDoc (
		Int int32,
		Str varchar NOT NULL,
		Bool bool NOT NULL,
		Float32 float32,
		Bytes bytes NOT NULL,
		UNIQUEFIELD Int
	);

	TABLE DocConstraintsString INHERITS CDoc (
		Str varchar,
		Int int32,
		UNIQUEFIELD Str
	);

	TABLE Config INHERITS Singleton (
		Fld1 varchar NOT NULL
	);

	TABLE cdoc1 INHERITS CDoc ();

	TABLE cdoc2 INHERITS CDoc (
		field1 ref, -- war RecordID, should be denied to create RecordID field -> ref type for now
		field2 ref(cdoc1, department),
		field3 ref
	);

	TABLE odoc1 INHERITS ODoc (
		orecord1 TABLE orecord1(
			orecord2 TABLE orecord2()
		)
	);

	TABLE odoc2 INHERITS ODoc (
		refToODoc1 ref(odoc1),
		refToORecord1 ref(orecord1),
		refToORecord2 ref(orecord2),
		refToAny ref,
		refToCDoc1 ref(cdoc1),
		refToCDoc1OrODoc1 ref(cdoc1, odoc1)
	);

	TYPE RatedQryParams (
		Fld text
	);

	TYPE RatedQryResult(
		Fld text -- not used
	);

	TYPE RatedCmdParams (
		Fld text
	);

	TYPE MockQryParams (
		Input text NOT NULL
	);

	TYPE MockQryResult (
		Res text NOT NULL
	);

	TYPE MockCmdParams(
		Input text NOT NULL
	);

	TYPE TestCmdParams (
		Arg1 int32 NOT NULL
	);

	TYPE TestCmdResult (
		Int int32 NOT NULL,
		Str text
	);

	VIEW View (
		ViewIntFld int32 NOT NULL,
		ViewStrFld text NOT NULL,
		ViewByteFld bytes(512),
		PRIMARY KEY ((ViewIntFld), ViewStrFld)
	) AS RESULT OF ProjDummy;

	EXTENSION ENGINE BUILTIN (
		QUERY RatedQry(RatedQryParams) RETURNS RatedQryResult;
		QUERY MockQry(MockQryParams) RETURNS MockQryResult;

		COMMAND RatedCmd(RatedCmdParams);
		COMMAND MockCmd(MockCmdParams);
		COMMAND TestCmd(TestCmdParams) RETURNS TestCmdResult;

		COMMAND CmdODocOne(odoc1);
		COMMAND CmdODocTwo(odoc2, UNLOGGED odoc2);
		PROJECTOR ProjDummy AFTER INSERT ON (CRecord) INTENTS(View(View)); -- does nothing, only to define view.app1pkg.View
	);
);
