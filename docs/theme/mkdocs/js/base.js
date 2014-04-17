$(document).ready(function ()
{

  // Tipue Search activation
  $('#tipue_search_input').tipuesearch({
    'mode': 'json',
    'contentLocation': '/search_content.json'
  });

  prettyPrint();

  // Resizing
  resizeMenuDropdown();
  checkToScrollTOC();
  $(window).resize(function() {
    if(this.resizeTO)
    {
      clearTimeout(this.resizeTO);
    }
    this.resizeTO = setTimeout(function ()
    {
      resizeMenuDropdown();
      checkToScrollTOC();
    }, 500);
  });

  /* Auto scroll */
  $('#nav_menu').scrollToFixed({
    dontSetWidth: true,
  });

  /* Toggle TOC view for Mobile */
  $('#toc_table').on('click', function ()
  {
    if ( $(window).width() <= 991 )
    {
      $('#toc_table > #toc_navigation').slideToggle();
    }
  })

  /* Follow TOC links (ScrollSpy) */
  $('body').scrollspy({
    target: '#toc_table',
  });

  /* Prevent disabled link clicks */
  $("li.disabled a").click(function ()
  {
    event.preventDefault();
  });

});

function resizeMenuDropdown ()
{
  $('.dd_menu > .dd_submenu').css("max-height", ($('body').height() - 160) + 'px');
}

// https://github.com/bigspotteddog/ScrollToFixed
function checkToScrollTOC ()
{
  if ( $(window).width() >= 768 )
  {
    if ( ($('#toc_table').height() + 100) >= $(window).height() )
    {
      $('#toc_table').trigger('detach.ScrollToFixed');
      $('#toc_navigation > li.active').removeClass('active');
    }
    else
    {
      $('#toc_table').scrollToFixed({
        marginTop: $('#nav_menu').height() + 14,
        limit: function () { return $('#footer').offset().top - 450; },
        zIndex: 1,
        minWidth: 768,
        removeOffsets: true,
      });
    }
  }
}