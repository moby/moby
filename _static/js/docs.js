
$(function(){

    // init multi-vers stuff
    $('.tabswitcher').each(function(i, multi_vers){
        var tabs = $('<ul></ul>');
        $(multi_vers).prepend(tabs);
        $(multi_vers).children('.tab').each(function(j, vers_content){
            vers = $(vers_content).children(':first').text();
            var id = 'multi_vers_' + '_' + i + '_' + j;
            $(vers_content).attr('id', id);
            $(tabs).append('<li><a href="#' + id + '">' + vers + '</a></li>');
        });
    });
    $( ".tabswitcher" ).tabs();
    
    // sidebar acordian-ing
    // don't apply on last object (it should be the FAQ)
   $('nav > ul > li > a').not(':last').click(function(){
	if ($(this).parent().hasClass('current')) {
	    $(this).parent().children('ul').slideUp(200, function() {
		$(this).parent().removeClass('current'); // toggle after effect
	    });
	} else {
	    $('nav > ul > li > ul').slideUp(100);
	    var current = $(this);
	    setTimeout(function() {      
		$('nav > ul > li').removeClass('current');
		current.parent().addClass('current'); // toggle before effect
		current.parent().children('ul').hide();
		current.parent().children('ul').slideDown(200);
	    }, 100);
	}
	return false;
     });
  
});